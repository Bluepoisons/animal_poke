package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	// HeaderRequestID 请求追踪 ID。
	HeaderRequestID = "X-Request-ID"
	// ContextRequestID gin context key。
	ContextRequestID = "request_id"
	// ContextDeviceID gin context key（鉴权后填充）。
	ContextDeviceID = "device_id"
)

// RequestID 注入/回传 X-Request-ID。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader(HeaderRequestID)
		if rid == "" {
			rid = uuid.NewString()
		}
		c.Set(ContextRequestID, rid)
		c.Writer.Header().Set(HeaderRequestID, rid)
		c.Next()
	}
}

// GetRequestID 从 context 取 request id。
func GetRequestID(c *gin.Context) string {
	if v, ok := c.Get(ContextRequestID); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// Logger 记录每次 HTTP 请求（结构化字段，含 request_id；不记录 Token/Key/照片）。
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		c.Next()

		// device_id 脱敏：仅保留前后各 4 位
		deviceID, _ := c.Get(ContextDeviceID)
		deviceStr := ""
		if s, ok := deviceID.(string); ok && s != "" {
			deviceStr = maskID(s)
		}

		attrs := []any{
			"method", c.Request.Method,
			"path", path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"ip", c.ClientIP(),
			"request_id", GetRequestID(c),
			"bytes_out", c.Writer.Size(),
		}
		if deviceStr != "" {
			attrs = append(attrs, "device_id", deviceStr)
		}
		status := c.Writer.Status()
		switch {
		case status >= 500:
			slog.Error("http", attrs...)
		case status >= 400:
			slog.Warn("http", attrs...)
		default:
			slog.Info("http", attrs...)
		}

		// Metrics: Gin route template only (FullPath). Unmatched → "unknown".
		// Never use raw URL.Path — that enables high-cardinality DoS.
		ObserveHTTP(c.Request.Method, c.FullPath(), status, time.Since(start))
	}
}

func maskID(s string) string {
	if len(s) <= 8 {
		return "***"
	}
	return s[:4] + "…" + s[len(s)-4:]
}
