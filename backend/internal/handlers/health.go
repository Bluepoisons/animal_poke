// Package handlers 健康检查 / 时间 / readiness。
package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"animalpoke/backend/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Health liveness：进程存活（兼容旧路径 /health）。
func Health() gin.HandlerFunc {
	return Livez()
}

// Livez 仅判断进程存活，不检查依赖。
func Livez() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"time":   time.Now().UTC().Format(time.RFC3339),
		})
	}
}

// ReadyDeps readiness 依赖。
type ReadyDeps struct {
	DB          *gorm.DB
	ReadyErrors []string
	AppEnv      string
	// SchemaOK 为 false 时表示迁移版本不匹配（可选）。
	SchemaOK *bool
}

// ReadyChecker 运行时可更新的 readiness 状态（依赖恢复后无需重启）。
type ReadyChecker struct {
	mu          sync.RWMutex
	db          *gorm.DB
	readyErrors []string
	appEnv      string
	schemaOK    *bool
}

// NewReadyChecker 构造。
func NewReadyChecker(deps ReadyDeps) *ReadyChecker {
	return &ReadyChecker{
		db:          deps.DB,
		readyErrors: append([]string(nil), deps.ReadyErrors...),
		appEnv:      deps.AppEnv,
		schemaOK:    deps.SchemaOK,
	}
}

// SetDB 更新 DB 句柄（连接恢复时可调用）。
func (r *ReadyChecker) SetDB(db *gorm.DB) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.db = db
}

// SetReadyErrors 更新配置错误列表。
func (r *ReadyChecker) SetReadyErrors(errs []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.readyErrors = append([]string(nil), errs...)
}

// Snapshot 返回当前依赖快照。
func (r *ReadyChecker) Snapshot() ReadyDeps {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return ReadyDeps{
		DB:          r.db,
		ReadyErrors: append([]string(nil), r.readyErrors...),
		AppEnv:      r.appEnv,
		SchemaOK:    r.schemaOK,
	}
}

// evaluateReady 统一 readiness 判定。
// db 失败时输出 reason=cert|auth|pool|network|unavailable，不回传原始错误/密钥。
func evaluateReady(deps ReadyDeps) (ready bool, details gin.H) {
	ready = true
	details = gin.H{"app_env": deps.AppEnv}

	if deps.DB != nil {
		sqlDB, err := deps.DB.DB()
		if err != nil {
			ready = false
			details["db"] = "down"
			details["db_reason"] = "pool"
		} else if pingErr := sqlDB.Ping(); pingErr != nil {
			ready = false
			details["db"] = "down"
			details["db_reason"] = config.ClassifyDBError(pingErr)
		} else {
			details["db"] = "up"
		}
	} else {
		details["db"] = "unavailable"
		details["db_reason"] = "unavailable"
		// 生产无 DB 视为未就绪；开发允许降级
		if deps.AppEnv == "production" || deps.AppEnv == "prod" {
			ready = false
		}
	}

	if deps.SchemaOK != nil && !*deps.SchemaOK {
		ready = false
		details["schema"] = "mismatch"
	} else if deps.SchemaOK != nil {
		details["schema"] = "ok"
	}

	if len(deps.ReadyErrors) > 0 {
		ready = false
		details["config_errors"] = deps.ReadyErrors
	}
	details["ready"] = ready
	return ready, details
}

// Ready readiness：配置与依赖（兼容 /ready）。
func Ready(deps ReadyDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ready, details := evaluateReady(deps)
		status := http.StatusOK
		if !ready {
			status = http.StatusServiceUnavailable
		}
		c.JSON(status, details)
	}
}

// Readyz 动态 readiness，支持依赖恢复后无需重启。
func Readyz(checker *ReadyChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		ready, details := evaluateReady(checker.Snapshot())
		status := http.StatusOK
		if !ready {
			status = http.StatusServiceUnavailable
		}
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
