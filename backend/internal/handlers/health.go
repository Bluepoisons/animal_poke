// Package handlers 健康检查 / 时间 / readiness。
package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Health liveness：进程存活。
func Health() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"time":   time.Now().UTC().Format(time.RFC3339),
		})
	}
}

// ReadyDeps readiness 依赖。
type ReadyDeps struct {
	DB           *gorm.DB
	ReadyErrors  []string
	AppEnv       string
}

// Ready readiness：配置与依赖。
func Ready(deps ReadyDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ready := true
		details := gin.H{"app_env": deps.AppEnv}
		if deps.DB != nil {
			sqlDB, err := deps.DB.DB()
			if err != nil || sqlDB.Ping() != nil {
				ready = false
				details["db"] = "down"
			} else {
				details["db"] = "up"
			}
		} else {
			details["db"] = "unavailable"
			// 开发允许无 DB；生产由配置错误体现
		}
		if len(deps.ReadyErrors) > 0 {
			ready = false
			details["config_errors"] = deps.ReadyErrors
		}
		status := http.StatusOK
		if !ready {
			status = http.StatusServiceUnavailable
		}
		details["ready"] = ready
		c.JSON(status, details)
	}
}

// TimeHandler 可信时间 API。
type TimeHandler struct {
	secret string
}

// NewTimeHandler 构造。
func NewTimeHandler(secret string) *TimeHandler {
	return &TimeHandler{secret: secret}
}

// GetTime GET /api/v1/time
func (h *TimeHandler) GetTime(c *gin.Context) {
	now := time.Now().UTC()
	nonce := uuid.NewString()
	payload := nonce + "|" + now.Format(time.RFC3339Nano)
	mac := hmac.New(sha256.New, []byte(h.secret))
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))
	c.Header("X-Server-Time", now.Format(time.RFC3339Nano))
	c.JSON(http.StatusOK, gin.H{
		"server_time": now.Format(time.RFC3339Nano),
		"unix_ms":     now.UnixMilli(),
		"nonce":       nonce,
		"signature":   sig,
		"expires_in":  60,
	})
}
