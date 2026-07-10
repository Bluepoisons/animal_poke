package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
	"time"

	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/repo"

	"github.com/gin-gonic/gin"
)

const (
	// HeaderIdempotencyKey 标准幂等头。
	HeaderIdempotencyKey = "Idempotency-Key"
	// DefaultIdempotencyTTL 完成记录保留时间。
	DefaultIdempotencyTTL = 24 * time.Hour
	// ProcessingTimeout 处理中超时后允许重试。
	ProcessingTimeout = 2 * time.Minute
	// MaxCachedBodyBytes 缓存响应上限。
	MaxCachedBodyBytes = 1 << 20 // 1MiB
)

// bodyWriter 捕获响应。
type bodyWriter struct {
	gin.ResponseWriter
	buf        *bytes.Buffer
	statusCode int
}

func (w *bodyWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *bodyWriter) Write(b []byte) (int, error) {
	if w.buf != nil && w.buf.Len() < MaxCachedBodyBytes {
		remain := MaxCachedBodyBytes - w.buf.Len()
		if len(b) > remain {
			w.buf.Write(b[:remain])
		} else {
			w.buf.Write(b)
		}
	}
	return w.ResponseWriter.Write(b)
}

// Idempotency 对指定路由启用服务端幂等。
// route 使用逻辑名（如 vision.detect / value.generate / sync.animal）。
func Idempotency(store *repo.IdempotencyRepo, route string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if store == nil {
			c.Next()
			return
		}
		key := strings.TrimSpace(c.GetHeader(HeaderIdempotencyKey))
		if key == "" {
			// 无 key：放行（兼容旧客户端）；生产 AI 路由可在上层强制
			c.Next()
			return
		}
		if len(key) > 128 {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Idempotency-Key too long", "reason_code": "idempotency_key_invalid"})
			return
		}
		deviceID := GetDeviceID(c)
		if deviceID == "" {
			deviceID = "anonymous"
		}

		// 读取并恢复 body 以计算摘要
		var bodyBytes []byte
		if c.Request.Body != nil {
			bodyBytes, _ = io.ReadAll(io.LimitReader(c.Request.Body, 6<<20))
			c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}
		reqHash := hashRequest(c.Request.Method, c.FullPath(), bodyBytes)

		rec, created, err := store.BeginOrGet(deviceID, route, key, reqHash, DefaultIdempotencyTTL)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "idempotency store failed"})
			return
		}

		if !created {
			// 已有记录
			if rec.RequestHash != reqHash {
				c.AbortWithStatusJSON(http.StatusConflict, gin.H{
					"error":       "idempotency key reuse with different request",
					"reason_code": "idempotency_conflict",
				})
				return
			}
			switch rec.Status {
			case "completed":
				c.Header("X-Idempotency-Replayed", "true")
				ct := "application/json; charset=utf-8"
				c.Data(rec.HTTPStatus, ct, []byte(rec.ResponseBody))
				c.Abort()
				return
			case "failed":
				// 5xx 失败允许重试：删除后重新执行
				_ = store.Delete(deviceID, route, key)
				// fallthrough to re-create
				rec, created, err = store.BeginOrGet(deviceID, route, key, reqHash, DefaultIdempotencyTTL)
				if err != nil {
					c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "idempotency store failed"})
					return
				}
				if !created && rec.Status == "completed" {
					c.Header("X-Idempotency-Replayed", "true")
					c.Data(rec.HTTPStatus, "application/json; charset=utf-8", []byte(rec.ResponseBody))
					c.Abort()
					return
				}
				if !created && rec.Status == "processing" {
					// still processing by another
					if time.Since(rec.UpdatedAt) < ProcessingTimeout {
						c.AbortWithStatusJSON(http.StatusConflict, gin.H{
							"error":       "request in progress",
							"reason_code": "idempotency_in_progress",
						})
						return
					}
					_ = store.Delete(deviceID, route, key)
					_, _, _ = store.BeginOrGet(deviceID, route, key, reqHash, DefaultIdempotencyTTL)
				}
			case "processing":
				if time.Since(rec.UpdatedAt) < ProcessingTimeout {
					c.AbortWithStatusJSON(http.StatusConflict, gin.H{
						"error":       "request in progress",
						"reason_code": "idempotency_in_progress",
					})
					return
				}
				// 超时：允许接管
				_ = store.Delete(deviceID, route, key)
				_, _, _ = store.BeginOrGet(deviceID, route, key, reqHash, DefaultIdempotencyTTL)
			default:
				// unknown status — proceed carefully
			}
		}

		// 捕获响应
		buf := &bytes.Buffer{}
		bw := &bodyWriter{ResponseWriter: c.Writer, buf: buf, statusCode: 200}
		c.Writer = bw
		c.Next()

		status := bw.statusCode
		if status == 0 {
			status = c.Writer.Status()
		}
		body := buf.String()

		// 缓存策略：2xx/4xx 缓存；5xx 标记 failed 允许重试
		cacheable := true
		if status >= 500 {
			cacheable = true // store as failed
		}
		if len(body) > MaxCachedBodyBytes {
			body = body[:MaxCachedBodyBytes]
		}
		_ = store.Complete(deviceID, route, key, status, body, cacheable)
		_ = rec // silence
	}
}

func hashRequest(method, path string, body []byte) string {
	h := sha256.New()
	h.Write([]byte(method))
	h.Write([]byte("|"))
	h.Write([]byte(path))
	h.Write([]byte("|"))
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

// Ensure models import used for godoc linkage in package.
var _ = models.IdempotencyRecord{}
