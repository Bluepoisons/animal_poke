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

// RequireIdempotencyKey rejects write requests that cannot be replayed safely.
func RequireIdempotencyKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.TrimSpace(c.GetHeader(HeaderIdempotencyKey)) == "" {
			AbortBadRequest(c, "idempotency_key_required", "Idempotency-Key is required", nil)
			return
		}
		c.Next()
	}
}

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
			AbortBadRequest(c, "idempotency_key_invalid", "Idempotency-Key too long", nil)
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
			AbortInternal(c, "idempotency_store_failed", "idempotency store failed")
			return
		}

		if !created {
			// 已有记录
			if rec.RequestHash != reqHash {
				AbortConflict(c, "idempotency_conflict", "idempotency key reuse with different request", nil)
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
				// 5xx 失败允许一个请求通过 CAS 获取新执行权。
				rec, created, err = store.TryTakeover(rec, reqHash, DefaultIdempotencyTTL)
				if err != nil {
					AbortInternal(c, "idempotency_store_failed", "idempotency store failed")
					return
				}
				if !created {
					AbortConflict(c, "idempotency_in_progress", "request in progress", nil)
					return
				}
			case "processing":
				if time.Since(rec.UpdatedAt) < ProcessingTimeout {
					AbortConflict(c, "idempotency_in_progress", "request in progress", nil)
					return
				}
				// 超时：仅一个请求可通过 CAS 接管。
				rec, created, err = store.TryTakeover(rec, reqHash, DefaultIdempotencyTTL)
				if err != nil {
					AbortInternal(c, "idempotency_store_failed", "idempotency store failed")
					return
				}
				if !created {
					AbortConflict(c, "idempotency_in_progress", "request in progress", nil)
					return
				}
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

		// Transient client-facing responses must not outlive their retry window.
		// In particular, caching 429 for 24h would replay yesterday's quota error
		// after the daily counter has reset. 5xx remains recorded as failed so a
		// later request can take over safely.
		cacheable := status != http.StatusRequestTimeout &&
			status != http.StatusConflict &&
			status != http.StatusTooEarly &&
			status != http.StatusTooManyRequests
		if len(body) > MaxCachedBodyBytes {
			body = body[:MaxCachedBodyBytes]
		}
		_, _ = store.CompleteClaim(rec, status, body, cacheable)
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
