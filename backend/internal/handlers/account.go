// Account binding / login / logout / device revoke handlers (AP-055).
package handlers

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/repo"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AccountHandler 账号绑定与设备迁移。
type AccountHandler struct {
	deviceRepo  *repo.DeviceRepo
	accountRepo *repo.AccountRepo
	jwtSecret   string
	jwtTTL      time.Duration
	refreshTTL  time.Duration
	issuer      string
	audience    string
}

// NewAccountHandler 构造。
func NewAccountHandler(deviceRepo *repo.DeviceRepo, accountRepo *repo.AccountRepo, jwtSecret string, jwtTTL time.Duration, issuer, audience string) *AccountHandler {
	return &AccountHandler{
		deviceRepo:  deviceRepo,
		accountRepo: accountRepo,
		jwtSecret:   jwtSecret,
		jwtTTL:      jwtTTL,
		refreshTTL:  30 * 24 * time.Hour,
		issuer:      issuer,
		audience:    audience,
	}
}

type bindRequest struct {
	Provider     string `json:"provider" binding:"required"` // email | mock_oauth
	Email        string `json:"email"`
	Password     string `json:"password"`
	OAuthSubject string `json:"oauth_subject"`
	OAuthToken   string `json:"oauth_token"`
	DisplayName  string `json:"display_name"`
}

type loginRequest struct {
	DeviceID     string `json:"device_id" binding:"required"`
	Provider     string `json:"provider" binding:"required"`
	Email        string `json:"email"`
	Password     string `json:"password"`
	OAuthSubject string `json:"oauth_subject"`
	OAuthToken   string `json:"oauth_token"`
}

type accountAuthResponse struct {
	Token        string           `json:"token"`
	ExpiresAt    string           `json:"expires_at"`
	TokenType    string           `json:"token_type"`
	AccountID    string           `json:"account_id,omitempty"`
	RefreshToken string           `json:"refresh_token,omitempty"` // 仅返回一次；服务端只存哈希
	Merge        *repo.MergeStats `json:"merge,omitempty"`
	Guest        bool             `json:"guest"`
}

type revokeDeviceRequest struct {
	DeviceID string `json:"device_id" binding:"required"`
}

// Bind POST /auth/bind — 当前设备绑定 email / mock OAuth（游客合并进账号）。
func (h *AccountHandler) Bind(c *gin.Context) {
	var req bindRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request", "reason_code": "bad_request"})
		return
	}
	deviceID := middleware.GetDeviceID(c)
	if deviceID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	provider, subject, secret, err := normalizeBindingInput(req.Provider, req.Email, req.Password, req.OAuthSubject, req.OAuthToken)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "reason_code": "invalid_binding"})
		return
	}

	dev, err := h.deviceRepo.Find(deviceID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
		return
	}
	if dev.Disabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "device disabled"})
		return
	}

	// 已有绑定身份？
	existing, err := h.accountRepo.FindBinding(provider, subject)
	var accountID string
	var mergeStats *repo.MergeStats
	if err == nil {
		// 绑定已存在 → 登录该账号并合并当前游客资产
		acc, aerr := h.accountRepo.EnsureAccountActive(existing.AccountID)
		if aerr != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "account disabled"})
			return
		}
		if !h.accountRepo.VerifyBindingCredential(existing, secret) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credential", "reason_code": "auth_failed"})
			return
		}
		accountID = acc.AccountID
		// 若设备已绑其他账号
		if dev.AccountID != "" && dev.AccountID != accountID {
			c.JSON(http.StatusConflict, gin.H{"error": "device bound to another account", "reason_code": "device_bound"})
			return
		}
		if dev.AccountID != accountID {
			mergeStats, err = h.accountRepo.MergeGuestIntoAccount(deviceID, accountID)
			if err != nil {
				slog.Error("merge guest failed", "err", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "merge failed"})
				return
			}
		}
	} else if err == gorm.ErrRecordNotFound {
		// 新绑定：若设备已有账号则复用，否则创建
		if dev.AccountID != "" {
			accountID = dev.AccountID
		} else {
			acc, cerr := h.accountRepo.CreateAccount(req.DisplayName)
			if cerr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "create account failed"})
				return
			}
			accountID = acc.AccountID
			mergeStats, err = h.accountRepo.MergeGuestIntoAccount(deviceID, accountID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "merge failed"})
				return
			}
		}
		var credHash string
		if provider == "email" {
			credHash, err = h.accountRepo.HashPassword(secret)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "hash failed"})
				return
			}
		} else {
			credHash = h.accountRepo.HashToken(secret)
		}
		if _, err := h.accountRepo.UpsertBinding(accountID, provider, subject, credHash); err != nil {
			if err == repo.ErrBindingConflict {
				c.JSON(http.StatusConflict, gin.H{"error": "binding conflict", "reason_code": "binding_conflict"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "bind failed"})
			return
		}
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "lookup failed"})
		return
	}

	_, refresh, err := h.accountRepo.LinkDevice(deviceID, accountID, "", h.refreshTTL)
	if err != nil {
		if err == repo.ErrAlreadyBound {
			c.JSON(http.StatusConflict, gin.H{"error": "device already bound", "reason_code": "device_bound"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "link device failed"})
		return
	}

	token, exp, err := h.issueToken(deviceID, accountID, dev.TokenVersion)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
		return
	}
	_ = h.accountRepo.TouchDevice(deviceID)
	c.JSON(http.StatusOK, accountAuthResponse{
		Token:        token,
		ExpiresAt:    exp.Format(time.RFC3339),
		TokenType:    "Bearer",
		AccountID:    accountID,
		RefreshToken: refresh,
		Merge:        mergeStats,
		Guest:        false,
	})
}

// Login POST /auth/login — 清除本地后用 email/mock OAuth 恢复，合并新游客设备。
func (h *AccountHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_id and provider required"})
		return
	}
	if !deviceIDPattern.MatchString(req.DeviceID) {
		if _, err := uuid.Parse(req.DeviceID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid device_id"})
			return
		}
	}
	provider, subject, secret, err := normalizeBindingInput(req.Provider, req.Email, req.Password, req.OAuthSubject, req.OAuthToken)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "reason_code": "invalid_binding"})
		return
	}

	binding, err := h.accountRepo.FindBinding(provider, subject)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credential", "reason_code": "auth_failed"})
		return
	}
	if !h.accountRepo.VerifyBindingCredential(binding, secret) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credential", "reason_code": "auth_failed"})
		return
	}
	acc, err := h.accountRepo.EnsureAccountActive(binding.AccountID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "account disabled"})
		return
	}

	dev, err := h.deviceRepo.FindOrCreate(req.DeviceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "device registration failed"})
		return
	}
	if dev.Disabled {
		// 若此前被 revoke，login 可重新启用（同账号）
		_ = h.deviceRepo.Enable(req.DeviceID)
		dev.Disabled = false
	}

	// 合并当前设备游客资产（若尚未归属该账号）
	var mergeStats *repo.MergeStats
	if dev.AccountID != acc.AccountID {
		mergeStats, err = h.accountRepo.MergeGuestIntoAccount(req.DeviceID, acc.AccountID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "merge failed"})
			return
		}
	}

	_, refresh, err := h.accountRepo.LinkDevice(req.DeviceID, acc.AccountID, "", h.refreshTTL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "link device failed"})
		return
	}
	// 重新读取 token version
	dev, _ = h.deviceRepo.Find(req.DeviceID)
	token, exp, err := h.issueToken(req.DeviceID, acc.AccountID, dev.TokenVersion)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
		return
	}
	c.JSON(http.StatusOK, accountAuthResponse{
		Token:        token,
		ExpiresAt:    exp.Format(time.RFC3339),
		TokenType:    "Bearer",
		AccountID:    acc.AccountID,
		RefreshToken: refresh,
		Merge:        mergeStats,
		Guest:        false,
	})
}

// Logout POST /auth/logout — 吊销本机 access/refresh（bump token_version）。
func (h *AccountHandler) Logout(c *gin.Context) {
	deviceID := middleware.GetDeviceID(c)
	if deviceID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	if err := h.accountRepo.LogoutDevice(deviceID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "logout failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "logged_out"})
}

// ListDevices GET /auth/devices
func (h *AccountHandler) ListDevices(c *gin.Context) {
	deviceID := middleware.GetDeviceID(c)
	dev, err := h.deviceRepo.Find(deviceID)
	if err != nil || dev.AccountID == "" {
		c.JSON(http.StatusOK, gin.H{"items": []any{}, "guest": true})
		return
	}
	list, err := h.accountRepo.ListDevices(dev.AccountID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list failed"})
		return
	}
	items := make([]gin.H, 0, len(list))
	for _, d := range list {
		items = append(items, gin.H{
			"device_id":    d.DeviceID,
			"device_label": repo.FormatDeviceLabel(d.DeviceID),
			"status":       d.Status,
			"linked_at":    d.LinkedAt.Format(time.RFC3339),
			"last_seen_at": formatTimePtr(d.LastSeenAt),
			"revoked_at":   formatTimePtr(d.RevokedAt),
			"current":      d.DeviceID == deviceID,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "account_id": dev.AccountID, "guest": false})
}

// RevokeDevice POST /auth/devices/revoke
func (h *AccountHandler) RevokeDevice(c *gin.Context) {
	var req revokeDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_id required"})
		return
	}
	deviceID := middleware.GetDeviceID(c)
	dev, err := h.deviceRepo.Find(deviceID)
	if err != nil || dev.AccountID == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "account required", "reason_code": "guest_mode"})
		return
	}
	if req.DeviceID == deviceID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot revoke current device", "reason_code": "self_revoke"})
		return
	}
	if err := h.accountRepo.RevokeDevice(dev.AccountID, req.DeviceID); err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "revoke failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "revoked", "device_id": req.DeviceID})
}

// GetAccount GET /auth/account
func (h *AccountHandler) GetAccount(c *gin.Context) {
	deviceID := middleware.GetDeviceID(c)
	dev, err := h.deviceRepo.Find(deviceID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"guest": true})
		return
	}
	if dev.AccountID == "" {
		c.JSON(http.StatusOK, gin.H{"guest": true, "device_id": deviceID})
		return
	}
	acc, err := h.accountRepo.FindAccount(dev.AccountID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"guest": true, "device_id": deviceID})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"guest":        false,
		"account_id":   acc.AccountID,
		"display_name": acc.DisplayName,
		"status":       acc.Status,
		"device_id":    deviceID,
	})
}

func (h *AccountHandler) issueToken(deviceID, accountID string, tokenVersion int) (string, time.Time, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(h.jwtTTL)
	claims := jwt.MapClaims{
		"device_id":     deviceID,
		"sub":           deviceID,
		"iss":           h.issuer,
		"aud":           h.audience,
		"iat":           now.Unix(),
		"exp":           expiresAt.Unix(),
		"jti":           uuid.NewString(),
		"token_version": tokenVersion,
	}
	if accountID != "" {
		claims["account_id"] = accountID
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(h.jwtSecret))
	return tokenStr, expiresAt, err
}

func normalizeBindingInput(provider, email, password, oauthSubject, oauthToken string) (prov, subject, secret string, err error) {
	prov = strings.ToLower(strings.TrimSpace(provider))
	switch prov {
	case "email":
		subject = repo.NormalizeEmail(email)
		if subject == "" || !strings.Contains(subject, "@") {
			return "", "", "", errBad("email required")
		}
		if len(password) < 8 {
			return "", "", "", errBad("password must be at least 8 characters")
		}
		secret = password
		return prov, subject, secret, nil
	case "mock_oauth":
		// 开发用 mock OAuth：subject + token（token 只存哈希）
		subject = strings.TrimSpace(oauthSubject)
		secret = strings.TrimSpace(oauthToken)
		if subject == "" || secret == "" {
			return "", "", "", errBad("oauth_subject and oauth_token required")
		}
		if len(subject) < 2 || len(secret) < 8 {
			return "", "", "", errBad("oauth credentials too short")
		}
		return prov, subject, secret, nil
	default:
		return "", "", "", errBad("unsupported provider")
	}
}

type badInputError string

func (e badInputError) Error() string { return string(e) }
func errBad(msg string) error         { return badInputError(msg) }

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}
