// Package handlers MB1: 设备鉴权处理函数。
package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"animalpoke/backend/internal/repo"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// AuthHandler 设备鉴权处理器。
type AuthHandler struct {
	deviceRepo *repo.DeviceRepo
	jwtSecret  string
	jwtTTL     time.Duration
}

// NewAuthHandler 构造 AuthHandler。
func NewAuthHandler(deviceRepo *repo.DeviceRepo, jwtSecret string, jwtTTL time.Duration) *AuthHandler {
	return &AuthHandler{
		deviceRepo: deviceRepo,
		jwtSecret:  jwtSecret,
		jwtTTL:     jwtTTL,
	}
}

type authRequest struct {
	DeviceID string `json:"device_id" binding:"required"`
}

type authResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

// DeviceAuth POST /auth/device 注册设备并签发 JWT Token。
func (h *AuthHandler) DeviceAuth(c *gin.Context) {
	var req authRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_id is required"})
		return
	}
	if req.DeviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_id cannot be empty"})
		return
	}

	dev, err := h.deviceRepo.FindOrCreate(req.DeviceID)
	if err != nil {
		slog.Error("设备注册失败", "device_id", req.DeviceID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "device registration failed"})
		return
	}

	now := time.Now()
	expiresAt := now.Add(h.jwtTTL)
	claims := jwt.MapClaims{
		"device_id": dev.DeviceID,
		"iat":       now.Unix(),
		"exp":       expiresAt.Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(h.jwtSecret))
	if err != nil {
		slog.Error("JWT 签发失败", "device_id", dev.DeviceID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
		return
	}

	slog.Info("设备鉴权成功", "device_id", dev.DeviceID)
	c.JSON(http.StatusOK, authResponse{
		Token:     tokenStr,
		ExpiresAt: expiresAt.Format(time.RFC3339),
	})
}
