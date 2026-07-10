// Package middleware — 统一错误 Envelope 与写回助手。
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ErrorResponse 标准 API 错误 Envelope。
// 所有 4xx/5xx JSON 错误应逐步收敛到此结构。
type ErrorResponse struct {
	Error      string         `json:"error"`
	ReasonCode string         `json:"reason_code"`
	RequestID  string         `json:"request_id,omitempty"`
	Retryable  bool           `json:"retryable"`
	Details    map[string]any `json:"details,omitempty"`
}

// NewErrorResponse 构造带 request_id 的错误体。
func NewErrorResponse(c *gin.Context, reason, message string, retryable bool, details map[string]any) ErrorResponse {
	return ErrorResponse{
		Error:      message,
		ReasonCode: reason,
		RequestID:  GetRequestID(c),
		Retryable:  retryable,
		Details:    details,
	}
}

// WriteError 写回标准错误 Envelope（不 Abort）。
func WriteError(c *gin.Context, status int, reason, message string, retryable bool, details map[string]any) {
	c.JSON(status, NewErrorResponse(c, reason, message, retryable, details))
}

// AbortJSON 写回标准错误 Envelope 并中止后续 handler。
func AbortJSON(c *gin.Context, status int, reason, message string, retryable bool, details map[string]any) {
	c.AbortWithStatusJSON(status, NewErrorResponse(c, reason, message, retryable, details))
}

// Common status helpers — 核心路径便捷封装。

// AbortUnauthorized 401。
func AbortUnauthorized(c *gin.Context, reason, message string) {
	if reason == "" {
		reason = "unauthorized"
	}
	if message == "" {
		message = "unauthorized"
	}
	AbortJSON(c, http.StatusUnauthorized, reason, message, false, nil)
}

// AbortForbidden 403。
func AbortForbidden(c *gin.Context, reason, message string) {
	if reason == "" {
		reason = "forbidden"
	}
	if message == "" {
		message = "forbidden"
	}
	AbortJSON(c, http.StatusForbidden, reason, message, false, nil)
}

// AbortBadRequest 400。
func AbortBadRequest(c *gin.Context, reason, message string, details map[string]any) {
	if reason == "" {
		reason = "bad_request"
	}
	if message == "" {
		message = "bad request"
	}
	AbortJSON(c, http.StatusBadRequest, reason, message, false, details)
}

// AbortTooLarge 413。
func AbortTooLarge(c *gin.Context, reason, message string) {
	if reason == "" {
		reason = "payload_too_large"
	}
	if message == "" {
		message = "request body too large"
	}
	AbortJSON(c, http.StatusRequestEntityTooLarge, reason, message, false, nil)
}

// AbortTooMany 429。
func AbortTooMany(c *gin.Context, reason, message string, retryAfterSec int, details map[string]any) {
	if reason == "" {
		reason = "rate_limited"
	}
	if message == "" {
		message = "rate limit exceeded"
	}
	if retryAfterSec > 0 {
		c.Header("Retry-After", itoa(retryAfterSec))
	}
	if details == nil {
		details = map[string]any{}
	}
	if retryAfterSec > 0 {
		details["retry_after"] = retryAfterSec
	}
	AbortJSON(c, http.StatusTooManyRequests, reason, message, true, details)
}

// AbortUnavailable 503。
func AbortUnavailable(c *gin.Context, reason, message string, retryAfterSec int) {
	if reason == "" {
		reason = "unavailable"
	}
	if message == "" {
		message = "service unavailable"
	}
	if retryAfterSec > 0 {
		c.Header("Retry-After", itoa(retryAfterSec))
	}
	details := map[string]any{}
	if retryAfterSec > 0 {
		details["retry_after"] = retryAfterSec
	}
	AbortJSON(c, http.StatusServiceUnavailable, reason, message, true, details)
}

// AbortInternal 500（不泄露内部细节）。
func AbortInternal(c *gin.Context, reason, message string) {
	if reason == "" {
		reason = "internal_error"
	}
	if message == "" {
		message = "internal server error"
	}
	AbortJSON(c, http.StatusInternalServerError, reason, message, true, nil)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
