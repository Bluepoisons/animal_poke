// Account security handlers (AP-079): email verify, password reset/change, unbind, reauth.
package handlers

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/repo"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type emailOnlyRequest struct {
	Email string `json:"email" binding:"required"`
}

type tokenOnlyRequest struct {
	Token string `json:"token" binding:"required"`
}

type passwordResetRequest struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

type passwordChangeRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required"`
}

type unbindRequest struct {
	Provider       string `json:"provider" binding:"required"`
	Subject        string `json:"subject"` // email 或 oauth subject；email 时可省略用绑定邮箱
	ReauthPassword string `json:"reauth_password"`
	ReauthToken    string `json:"reauth_token"`
}

type reauthRequest struct {
	Password string `json:"password" binding:"required"`
}

// RequestEmailVerify POST /auth/email/verify/request
// 反枚举：无论邮箱是否存在均 200 accepted。
func (h *AccountHandler) RequestEmailVerify(c *gin.Context) {
	var req emailOnlyRequest
	if err := middleware.BindStrictJSON(c, &req); err != nil {
		middleware.WriteBindError(c, err)
		return
	}
	email := repo.NormalizeEmail(req.Email)
	if email == "" || !strings.Contains(email, "@") {
		// 格式错误仍可 400（不泄露存在性）
		middleware.WriteError(c, http.StatusBadRequest, "invalid_email", "invalid email", false, nil)
		return
	}
	// 仅对未验证绑定签发；不向客户端区分结果
	if b, err := h.accountRepo.FindEmailBinding(email); err == nil && !b.Verified {
		if _, err := h.issueEmailVerifyToken(b.AccountID, email, b.ID); err != nil {
			slog.Error("issue email verify failed", "err", err)
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": "accepted"})
}

// VerifyEmail POST /auth/email/verify — 消费验证令牌。
func (h *AccountHandler) VerifyEmail(c *gin.Context) {
	var req tokenOnlyRequest
	if err := middleware.BindStrictJSON(c, &req); err != nil {
		middleware.WriteBindError(c, err)
		return
	}
	tok, err := h.accountRepo.ConsumeSecurityToken(strings.TrimSpace(req.Token), models.SecurityPurposeEmailVerify, "")
	if err != nil {
		h.writeSecurityTokenError(c, err)
		return
	}
	bindingID := tok.BindingID
	if bindingID == 0 {
		// 兼容：按 subject 找回
		if b, ferr := h.accountRepo.FindEmailBinding(tok.Subject); ferr == nil {
			bindingID = b.ID
		}
	}
	if bindingID == 0 {
		middleware.WriteError(c, http.StatusBadRequest, "token_invalid", "invalid or expired token", false, nil)
		return
	}
	if err := h.accountRepo.MarkBindingVerified(bindingID); err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "verify_failed", "verify failed", true, nil)
		return
	}
	_ = h.accountRepo.DB().Create(&models.AuditLog{
		DeviceID: middleware.GetDeviceID(c),
		Type:     "auth",
		Message:  "email_verified",
		Metadata: tok.AccountID,
		Status:   "closed",
	}).Error
	c.JSON(http.StatusOK, gin.H{"status": "verified", "account_id": tok.AccountID})
}

// ForgotPassword POST /auth/password/forgot — 反枚举 200。
func (h *AccountHandler) ForgotPassword(c *gin.Context) {
	var req emailOnlyRequest
	if err := middleware.BindStrictJSON(c, &req); err != nil {
		middleware.WriteBindError(c, err)
		return
	}
	email := repo.NormalizeEmail(req.Email)
	if email == "" || !strings.Contains(email, "@") {
		middleware.WriteError(c, http.StatusBadRequest, "invalid_email", "invalid email", false, nil)
		return
	}
	var debug string
	if b, err := h.accountRepo.FindEmailBinding(email); err == nil && b.Verified {
		// 仅已验证邮箱可重置
		_ = h.accountRepo.InvalidateSecurityTokens(b.AccountID, models.SecurityPurposePasswordReset)
		plain, _, cerr := h.accountRepo.CreateSecurityToken(models.SecurityPurposePasswordReset, b.AccountID, email, b.ID, h.passwordResetTTL)
		if cerr == nil {
			if h.mailer != nil {
				_ = h.mailer.SendSecurityMail(email, models.SecurityPurposePasswordReset, plain)
			}
			if h.exposeDebugTokens {
				debug = plain
			}
		}
	}
	resp := gin.H{"status": "accepted"}
	if debug != "" {
		resp["debug_security_token"] = debug
	}
	c.JSON(http.StatusOK, resp)
}

// ResetPassword POST /auth/password/reset
func (h *AccountHandler) ResetPassword(c *gin.Context) {
	var req passwordResetRequest
	if err := middleware.BindStrictJSON(c, &req); err != nil {
		middleware.WriteBindError(c, err)
		return
	}
	if len(req.NewPassword) < 8 {
		middleware.WriteError(c, http.StatusBadRequest, "invalid_password", "password must be at least 8 characters", false, nil)
		return
	}
	tok, err := h.accountRepo.ConsumeSecurityToken(strings.TrimSpace(req.Token), models.SecurityPurposePasswordReset, "")
	if err != nil {
		h.writeSecurityTokenError(c, err)
		return
	}
	hash, err := h.accountRepo.HashPassword(req.NewPassword)
	if err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "hash_failed", "hash failed", true, nil)
		return
	}
	bindingID := tok.BindingID
	if bindingID == 0 {
		if b, ferr := h.accountRepo.FindEmailBinding(tok.Subject); ferr == nil {
			bindingID = b.ID
		}
	}
	if bindingID == 0 {
		middleware.WriteError(c, http.StatusBadRequest, "token_invalid", "invalid or expired token", false, nil)
		return
	}
	if err := h.accountRepo.UpdateBindingPassword(bindingID, hash); err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "reset_failed", "reset failed", true, nil)
		return
	}
	// 改密后吊销全部会话
	if err := h.accountRepo.InvalidateAccountSessions(tok.AccountID); err != nil {
		slog.Error("invalidate sessions after reset failed", "err", err)
	}
	_ = h.accountRepo.InvalidateSecurityTokens(tok.AccountID, models.SecurityPurposePasswordReset)
	_ = h.accountRepo.DB().Create(&models.AuditLog{
		DeviceID: "",
		Type:     "auth",
		Message:  "password_reset",
		Metadata: tok.AccountID,
		Status:   "closed",
	}).Error
	c.JSON(http.StatusOK, gin.H{"status": "password_reset"})
}

// ChangePassword POST /auth/password/change — 需登录；吊销全部会话后为本机重签。
func (h *AccountHandler) ChangePassword(c *gin.Context) {
	var req passwordChangeRequest
	if err := middleware.BindStrictJSON(c, &req); err != nil {
		middleware.WriteBindError(c, err)
		return
	}
	if len(req.NewPassword) < 8 {
		middleware.WriteError(c, http.StatusBadRequest, "invalid_password", "password must be at least 8 characters", false, nil)
		return
	}
	deviceID := middleware.GetDeviceID(c)
	dev, err := h.deviceRepo.Find(deviceID)
	if err != nil || dev.AccountID == "" {
		middleware.WriteError(c, http.StatusForbidden, "account_required", "account required", false, nil)
		return
	}
	binding, err := h.accountRepo.FindVerifiedEmailBinding(dev.AccountID)
	if err != nil {
		middleware.WriteError(c, http.StatusForbidden, "email_required", "verified email binding required", false, nil)
		return
	}
	if !h.accountRepo.VerifyBindingCredential(binding, req.CurrentPassword) {
		middleware.WriteError(c, http.StatusUnauthorized, "auth_failed", "invalid credential", false, nil)
		return
	}
	hash, err := h.accountRepo.HashPassword(req.NewPassword)
	if err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "hash_failed", "hash failed", true, nil)
		return
	}
	if err := h.accountRepo.UpdateBindingPassword(binding.ID, hash); err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "change_failed", "change failed", true, nil)
		return
	}
	if err := h.accountRepo.InvalidateAccountSessions(dev.AccountID); err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "session_revoke_failed", "session revoke failed", true, nil)
		return
	}
	// 重读 token_version 后为本机签发新 access + refresh
	dev, err = h.deviceRepo.Find(deviceID)
	if err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "device_lookup_failed", "device lookup failed", true, nil)
		return
	}
	_, refresh, err := h.accountRepo.LinkDevice(deviceID, dev.AccountID, "", h.refreshAbsoluteTTL)
	if err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "link_device_failed", "link device failed", true, nil)
		return
	}
	token, exp, err := h.issueToken(deviceID, dev.AccountID, dev.TokenVersion)
	if err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "token_generation_failed", "token generation failed", true, nil)
		return
	}
	_ = h.accountRepo.DB().Create(&models.AuditLog{
		DeviceID: deviceID,
		Type:     "auth",
		Message:  "password_changed",
		Metadata: dev.AccountID,
		Status:   "closed",
	}).Error
	c.JSON(http.StatusOK, accountAuthResponse{
		Token:        token,
		ExpiresAt:    exp.Format(time.RFC3339),
		TokenType:    "Bearer",
		AccountID:    dev.AccountID,
		RefreshToken: refresh,
		Guest:        false,
	})
}

// UnbindProvider POST /auth/unbind — 解绑；禁止移除最后一种恢复方式。
func (h *AccountHandler) UnbindProvider(c *gin.Context) {
	var req unbindRequest
	if err := middleware.BindStrictJSON(c, &req); err != nil {
		middleware.WriteBindError(c, err)
		return
	}
	deviceID := middleware.GetDeviceID(c)
	dev, err := h.deviceRepo.Find(deviceID)
	if err != nil || dev.AccountID == "" {
		middleware.WriteError(c, http.StatusForbidden, "account_required", "account required", false, nil)
		return
	}
	if !h.verifyReauth(dev.AccountID, req.ReauthPassword, req.ReauthToken) {
		middleware.WriteError(c, http.StatusForbidden, "reauth_required", "re-authentication required", false, nil)
		return
	}
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	subject := strings.TrimSpace(req.Subject)
	if provider == "email" {
		if subject == "" {
			if b, err := h.accountRepo.FindVerifiedEmailBinding(dev.AccountID); err == nil {
				subject = b.ProviderSubject
			} else {
				// 允许解绑未验证邮箱：取任意 email 绑定
				list, _ := h.accountRepo.ListBindings(dev.AccountID)
				for _, b := range list {
					if b.Provider == "email" {
						subject = b.ProviderSubject
						break
					}
				}
			}
		} else {
			subject = repo.NormalizeEmail(subject)
		}
	}
	if provider == "" || subject == "" {
		middleware.WriteError(c, http.StatusBadRequest, "invalid_binding", "provider and subject required", false, nil)
		return
	}
	if err := h.accountRepo.DeleteBinding(dev.AccountID, provider, subject); err != nil {
		switch err {
		case repo.ErrLastRecoveryMethod:
			middleware.WriteError(c, http.StatusConflict, "last_recovery_method", "cannot remove last recovery method", false, nil)
			return
		case repo.ErrBindingNotFound, gorm.ErrRecordNotFound:
			middleware.WriteError(c, http.StatusNotFound, "binding_not_found", "binding not found", false, nil)
			return
		default:
			middleware.WriteError(c, http.StatusInternalServerError, "unbind_failed", "unbind failed", true, nil)
			return
		}
	}
	_ = h.accountRepo.DB().Create(&models.AuditLog{
		DeviceID: deviceID,
		Type:     "auth",
		Message:  "provider_unbound",
		Metadata: provider + ":" + subject,
		Status:   "closed",
	}).Error
	c.JSON(http.StatusOK, gin.H{"status": "unbound", "provider": provider})
}

// Reauth POST /auth/reauth — 签发短期 reauth 令牌供敏感操作使用。
func (h *AccountHandler) Reauth(c *gin.Context) {
	var req reauthRequest
	if err := middleware.BindStrictJSON(c, &req); err != nil {
		middleware.WriteBindError(c, err)
		return
	}
	deviceID := middleware.GetDeviceID(c)
	dev, err := h.deviceRepo.Find(deviceID)
	if err != nil || dev.AccountID == "" {
		middleware.WriteError(c, http.StatusForbidden, "account_required", "account required", false, nil)
		return
	}
	binding, err := h.accountRepo.FindVerifiedEmailBinding(dev.AccountID)
	if err != nil || !h.accountRepo.VerifyBindingCredential(binding, req.Password) {
		middleware.WriteError(c, http.StatusUnauthorized, "auth_failed", "invalid credential", false, nil)
		return
	}
	_ = h.accountRepo.InvalidateSecurityTokens(dev.AccountID, models.SecurityPurposeReauth)
	plain, row, err := h.accountRepo.CreateSecurityToken(models.SecurityPurposeReauth, dev.AccountID, binding.ProviderSubject, binding.ID, h.reauthTTL)
	if err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "reauth_failed", "reauth failed", true, nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"reauth_token": plain,
		"expires_at":   row.ExpiresAt.Format(time.RFC3339),
		"token_type":   "reauth",
	})
}

func (h *AccountHandler) verifyReauth(accountID, password, reauthToken string) bool {
	if strings.TrimSpace(reauthToken) != "" {
		if _, err := h.accountRepo.PeekSecurityToken(strings.TrimSpace(reauthToken), models.SecurityPurposeReauth, accountID); err == nil {
			return true
		}
	}
	if strings.TrimSpace(password) == "" {
		return false
	}
	b, err := h.accountRepo.FindVerifiedEmailBinding(accountID)
	if err != nil {
		return false
	}
	return h.accountRepo.VerifyBindingCredential(b, password)
}

func (h *AccountHandler) writeSecurityTokenError(c *gin.Context, err error) {
	switch err {
	case repo.ErrSecurityTokenUsed:
		middleware.WriteError(c, http.StatusConflict, "token_replay", "token already used", false, nil)
	case repo.ErrSecurityTokenExpired:
		middleware.WriteError(c, http.StatusUnauthorized, "token_expired", "token expired", false, nil)
	default:
		middleware.WriteError(c, http.StatusUnauthorized, "token_invalid", "invalid or expired token", false, nil)
	}
}
