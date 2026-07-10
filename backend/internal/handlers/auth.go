// Package handlers MB1: 设备鉴权处理函数。
package handlers

import (
	"log/slog"
	"net/http"
	"regexp"
	"time"

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
	DeviceID string `json:"device_id" binding:"required"`
}

type authResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
	TokenType string `json:"token_type"`
	AccountID string `json:"account_id,omitempty"`
	Guest     bool   `json:"guest"`
}

// DeviceAuth POST /auth/device 注册设备并签发 JWT Token。
func (h *AuthHandler) DeviceAuth(c *gin.Context) {
	var req authRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_id is required"})
		return
	}
	if !deviceIDPattern.MatchString(req.DeviceID) {
		// 也允许标准 UUID
		if _, err := uuid.Parse(req.DeviceID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "device_id must be UUID or 8-64 alphanumeric/_-"})
			return
		}
	}

	dev, err := h.deviceRepo.FindOrCreate(req.DeviceID)
	if err != nil {
		slog.Error("设备注册失败", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "device registration failed"})
		return
	}
	if dev.Disabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "device disabled"})
		return
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
	tokenStr, err := token.SignedString([]byte(h.jwtSecret))
	if err != nil {
		slog.Error("JWT 签发失败", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
		return
	}

	slog.Info("设备鉴权成功", "device_id", dev.DeviceID, "account_id", dev.AccountID)
	c.JSON(http.StatusOK, authResponse{
		Token:     tokenStr,
		ExpiresAt: expiresAt.Format(time.RFC3339),
		TokenType: "Bearer",
		AccountID: dev.AccountID,
		Guest:     dev.AccountID == "",
	})
}
