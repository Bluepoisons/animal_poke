// Package handlers MB1: 设备鉴权处理函数。
package handlers

import (
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/repo"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// deviceIDPattern UUID 或 8-64 位 [a-zA-Z0-9_-]
var deviceIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{8,64}$`)

// AuthHandler 设备鉴权处理器。
type AuthHandler struct {
	deviceRepo *repo.DeviceRepo
	jwtSecret  string
	jwtTTL     time.Duration
	issuer     string
	audience   string
}

// NewAuthHandler 构造 AuthHandler。
func NewAuthHandler(deviceRepo *repo.DeviceRepo, jwtSecret string, jwtTTL time.Duration) *AuthHandler {
	return &AuthHandler{
		deviceRepo: deviceRepo,
		jwtSecret:  jwtSecret,
		jwtTTL:     jwtTTL,
		issuer:     "animal-poke",
		audience:   "animal-poke-client",
	}
}

// NewAuthHandlerFull 完整构造。
func NewAuthHandlerFull(deviceRepo *repo.DeviceRepo, jwtSecret string, jwtTTL time.Duration, issuer, audience string) *AuthHandler {
	h := NewAuthHandler(deviceRepo, jwtSecret, jwtTTL)
	if issuer != "" {
		h.issuer = issuer
	}
	if audience != "" {
		h.audience = audience
	}
	return h
}

type authRequest struct {
	DeviceID           string `json:"device_id" binding:"required"`
	InstallationSecret string `json:"installation_secret"`
}

type authResponse struct {
	Token              string `json:"token"`
	ExpiresAt          string `json:"expires_at"`
	TokenType          string `json:"token_type"`
	AccountID          string `json:"account_id,omitempty"`
	Guest              bool   `json:"guest"`
	InstallationSecret string `json:"installation_secret,omitempty"`
}

// DeviceAuth POST /auth/device 注册设备并签发 JWT Token。
// 首次注册：生成 installation_secret，仅本次响应返回明文；后续换 Token 必须携带并校验。
func (h *AuthHandler) DeviceAuth(c *gin.Context) {
	var req authRequest
	if err := middleware.BindStrictJSON(c, &req); err != nil {
		middleware.WriteBindError(c, err)
		return
	}
	if !deviceIDPattern.MatchString(req.DeviceID) {
		// 也允许标准 UUID
		if _, err := uuid.Parse(req.DeviceID); err != nil {
			middleware.AbortBadRequest(c, "invalid_device_id", "device_id must be UUID or 8-64 alphanumeric/_-", nil)
			return
		}
	}

	dev, err := h.deviceRepo.FindOrCreate(req.DeviceID)
	if err != nil {
		slog.Error("设备注册失败", "err", err)
		middleware.AbortInternal(c, "device_registration_failed", "device registration failed")
		return
	}
	if dev.Disabled {
		middleware.AbortForbidden(c, "device_disabled", "device disabled")
		return
	}

	var returnedSecret string
	if dev.InstallationSecretHash == "" {
		// 首次注册（或历史设备升级路径）：生成并仅成功占用者拿到明文
		secret, salt, genErr := repo.GenerateInstallationSecret()
		if genErr != nil {
			slog.Error("生成 installation secret 失败", "err", genErr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "secret generation failed"})
			return
		}
		claimed, setErr := h.deviceRepo.SetInstallationSecret(dev.DeviceID, secret, salt)
		if setErr != nil {
			slog.Error("写入 installation secret 失败", "err", setErr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "secret persistence failed"})
			return
		}
		if !claimed {
			// 并发注册：其他请求已占用 secret，本请求必须证明持有
			if req.InstallationSecret == "" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "installation_secret required"})
				return
			}
			ok, vErr := h.deviceRepo.VerifyInstallationSecret(dev.DeviceID, req.InstallationSecret)
			if vErr != nil {
				slog.Error("校验 installation secret 失败", "err", vErr)
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": "secret verification unavailable"})
				return
			}
			if !ok {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid installation_secret"})
				return
			}
		} else {
			returnedSecret = secret
			// 刷新内存中的 hash 标记
			dev.InstallationSecretHash = repo.HashInstallationSecret(secret, salt)
			dev.InstallationSecretSalt = salt
		}
	} else {
		// 已知设备：仅凭 device_id 不足，必须证明持有 installation_secret
		if req.InstallationSecret == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "installation_secret required"})
			return
		}
		ok, vErr := h.deviceRepo.VerifyInstallationSecret(dev.DeviceID, req.InstallationSecret)
		if vErr != nil {
			slog.Error("校验 installation secret 失败", "err", vErr)
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "secret verification unavailable"})
			return
		}
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid installation_secret"})
			return
		}
	}

	now := time.Now().UTC()
	expiresAt := now.Add(h.jwtTTL)
	jti := uuid.NewString()
	claims := jwt.MapClaims{
		"device_id":     dev.DeviceID,
		"sub":           dev.DeviceID,
		"iss":           h.issuer,
		"aud":           h.audience,
		"iat":           now.Unix(),
		"exp":           expiresAt.Unix(),
		"jti":           jti,
		"token_version": dev.TokenVersion,
	}
	if dev.AccountID != "" {
		claims["account_id"] = dev.AccountID
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	// 可选 kid，便于密钥轮换与观测（v1 = 当前 JWT_SECRET）
	token.Header["kid"] = "v1"
	tokenStr, err := token.SignedString([]byte(h.jwtSecret))
	if err != nil {
		slog.Error("JWT 签发失败", "err", err)
		middleware.AbortInternal(c, "token_generation_failed", "token generation failed")
		return
	}

	slog.Info("设备鉴权成功", "device_id", dev.DeviceID, "account_id", dev.AccountID)
	c.JSON(http.StatusOK, authResponse{
		Token:              tokenStr,
		ExpiresAt:          expiresAt.Format(time.RFC3339),
		TokenType:          "Bearer",
		AccountID:          dev.AccountID,
		Guest:              dev.AccountID == "",
		InstallationSecret: returnedSecret,
	})
}
