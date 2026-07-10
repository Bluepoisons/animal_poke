// Package middleware — 严格 JSON 解码与按路由 Body 上限。
package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

// Body size classes（按路由类型分级）。
const (
	// MaxBodyDefault 普通 JSON（auth、value、consent 等）。
	MaxBodyDefault int64 = 64 << 10 // 64 KiB
	// MaxBodySyncBatch 批量同步。
	MaxBodySyncBatch int64 = 1 << 20 // 1 MiB
	// MaxBodyErrorReport 前端错误栈上报。
	MaxBodyErrorReport int64 = 16 << 10 // 16 KiB
	// MaxBodyReceipt 商业化回执。
	MaxBodyReceipt int64 = 256 << 10 // 256 KiB
	// MaxBodyGlobal 全局硬上限（略大于最大图片请求，兜底防 OOM）。
	MaxBodyGlobal int64 = 6 << 20 // 6 MiB
)

// Sentinel / classified bind errors.
var (
	// ErrTrailingJSON JSON 后仍有非空白内容。
	ErrTrailingJSON = errors.New("trailing data after JSON value")
	// ErrEmptyBody 请求体为空。
	ErrEmptyBody = errors.New("empty request body")
)

// BodyLimit 将 c.Request.Body 包装为 MaxBytesReader。
// 应在读取 body 前挂到路由/组上；可叠加（取更严者时先挂小的）。
func BodyLimit(maxBytes int64) gin.HandlerFunc {
	if maxBytes <= 0 {
		maxBytes = MaxBodyDefault
	}
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Set("max_body_bytes", maxBytes)
		c.Next()
	}
}

// GlobalBodyLimit 可选全局硬上限中间件。
func GlobalBodyLimit(maxBytes int64) gin.HandlerFunc {
	if maxBytes <= 0 {
		maxBytes = MaxBodyGlobal
	}
	return BodyLimit(maxBytes)
}

// BindStrictJSON 严格 JSON 绑定：
// - DisallowUnknownFields
// - 拒绝 trailing junk（EOF 后不得再有 token）
// - 依赖上游 MaxBytesReader 做 body 上限
// - 若目标带 binding tag，走 gin validator
func BindStrictJSON(c *gin.Context, dst any) error {
	if c.Request == nil || c.Request.Body == nil {
		return ErrEmptyBody
	}

	// 读完受限 body，便于分类 MaxBytesError 与 trailing 检测。
	// MaxBytesReader 超限时 ReadAll 返回 *http.MaxBytesError。
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return ErrEmptyBody
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}

	// 拒绝 trailing JSON / junk（仅允许尾部空白 → EOF）。
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return ErrTrailingJSON
	}

	if binding.Validator != nil {
		if err := binding.Validator.ValidateStruct(dst); err != nil {
			return err
		}
	}
	return nil
}

// WriteBindError 将 BindStrictJSON / MaxBytes 错误映射为标准 Envelope。
// 不泄露内部细节；413 / 400 分流。
func WriteBindError(c *gin.Context, err error) {
	if err == nil {
		return
	}
	if IsMaxBytesError(err) {
		AbortTooLarge(c, "payload_too_large", "request body too large")
		return
	}
	if errors.Is(err, ErrTrailingJSON) {
		AbortBadRequest(c, "trailing_json", "trailing data after JSON body", nil)
		return
	}
	if errors.Is(err, ErrEmptyBody) {
		AbortBadRequest(c, "empty_body", "request body is required", nil)
		return
	}
	if isUnknownFieldError(err) {
		field := extractUnknownField(err)
		details := map[string]any{}
		if field != "" {
			details["field"] = field
		}
		AbortBadRequest(c, "unknown_field", "unknown field in JSON body", details)
		return
	}
	// 语法错误 / 类型错误 / binding 校验
	msg := "invalid JSON body"
	reason := "bad_request"
	if isJSONSyntaxError(err) {
		reason = "malformed_json"
		msg = "malformed JSON body"
	}
	AbortBadRequest(c, reason, msg, nil)
}

// IsMaxBytesError 判断是否因 body 超限失败。
func IsMaxBytesError(err error) bool {
	if err == nil {
		return false
	}
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		return true
	}
	// 兼容部分包装文案
	s := err.Error()
	return strings.Contains(s, "http: request body too large") ||
		strings.Contains(s, "request body too large")
}

func isUnknownFieldError(err error) bool {
	if err == nil {
		return false
	}
	return strings.HasPrefix(err.Error(), "json: unknown field")
}

func extractUnknownField(err error) string {
	// json: unknown field "foo"
	s := err.Error()
	const p = `json: unknown field "`
	if !strings.HasPrefix(s, p) {
		return ""
	}
	rest := s[len(p):]
	if i := strings.IndexByte(rest, '"'); i >= 0 {
		return rest[:i]
	}
	return ""
}

func isJSONSyntaxError(err error) bool {
	var se *json.SyntaxError
	var te *json.UnmarshalTypeError
	return errors.As(err, &se) || errors.As(err, &te)
}
