package handlers

import (
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"animalpoke/backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

// errorReportRequest is the shared wire schema for POST /api/v1/errors/report.
// Must stay aligned with frontend ErrorReportPayload (strict JSON binding).
type errorReportRequest struct {
	Message   string            `json:"message"`
	Stack     string            `json:"stack"`
	Component string            `json:"component"`
	Route     string            `json:"route"`
	Release   string            `json:"release"`
	Level     string            `json:"level"`
	RequestID string            `json:"request_id"`
	Extra     map[string]string `json:"extra"`
}

// ErrorReportHandler 接收前端错误（已鉴权），脱敏后记录。
type ErrorReportHandler struct{}

func NewErrorReportHandler() *ErrorReportHandler { return &ErrorReportHandler{} }

// Report POST /api/v1/errors/report
func (h *ErrorReportHandler) Report(c *gin.Context) {
	// Body 上限由路由 BodyLimit(MaxBodyErrorReport) 控制。
	var req errorReportRequest
	if err := middleware.BindStrictJSON(c, &req); err != nil {
		middleware.WriteBindError(c, err)
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		middleware.AbortBadRequest(c, "missing_message", "message required", nil)
		return
	}
	// 脱敏：截断、剔除疑似 token/key/坐标/照片
	msg := redact(truncate(req.Message, 500))
	stack := redact(truncate(req.Stack, 2000))
	extra := redactExtra(req.Extra)
	device := middleware.GetDeviceID(c)
	if len(device) > 8 {
		device = device[:4] + "…" + device[len(device)-4:]
	}
	serverRID := middleware.GetRequestID(c)
	clientRID := truncate(req.RequestID, 64)
	if clientRID == "" {
		clientRID = serverRID
	}
	slog.Warn("client_error",
		"request_id", serverRID,
		"client_request_id", clientRID,
		"device", device,
		"message", msg,
		"component", truncate(req.Component, 80),
		"route", truncate(req.Route, 120),
		"release", truncate(req.Release, 64),
		"level", truncate(req.Level, 16),
		"stack", stack,
		"extra", extra,
	)
	c.JSON(http.StatusAccepted, gin.H{
		"accepted":   true,
		"request_id": serverRID,
	})
}

func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n]) + "…"
}

var (
	reBearer     = regexp.MustCompile(`(?i)bearer\s+[a-z0-9._\-+=/]+`)
	reJWT        = regexp.MustCompile(`\beyJ[a-zA-Z0-9_-]{10,}\.[a-zA-Z0-9_-]{10,}\.[a-zA-Z0-9_-]{10,}\b`)
	reDataImage  = regexp.MustCompile(`(?i)data:image/[a-z0-9+.\-]+;base64,[a-z0-9+/=\s]+`)
	reCoordsPair = regexp.MustCompile(`\b-?\d{1,3}\.\d{4,}\s*[,/]\s*-?\d{1,3}\.\d{4,}\b`)
	reLatLngKey  = regexp.MustCompile(`(?i)\b(lat(?:itude)?|lng|lon(?:gitude)?|coords?)\s*[:=]\s*-?\d+(\.\d+)?`)
	reSecretKey  = regexp.MustCompile(`(?i)(authorization|api[_-]?key|apikey|access_token|refresh_token|password|jwt|installation_secret)\s*[:=]\s*['"]?[^'"\s,;]+`)
)

func redact(s string) string {
	if s == "" {
		return s
	}
	out := s
	out = reBearer.ReplaceAllString(out, "Bearer [redacted]")
	out = reSecretKey.ReplaceAllString(out, "${1}=[redacted]")
	out = reJWT.ReplaceAllString(out, "[redacted-jwt]")
	out = reDataImage.ReplaceAllString(out, "[redacted-photo]")
	out = reCoordsPair.ReplaceAllString(out, "[redacted-coords]")
	out = reLatLngKey.ReplaceAllString(out, "${1}=[redacted]")

	lower := strings.ToLower(out)
	// Whole-string fallback when secret keywords remain without redaction markers
	for _, needle := range []string{"bearer ", "api_key", "apikey", "password", "authorization"} {
		if strings.Contains(lower, needle) && !strings.Contains(lower, "[redacted") {
			return "[redacted]"
		}
	}
	return out
}

func redactExtra(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	n := 0
	for k, v := range in {
		if n >= 12 {
			break
		}
		lk := strings.ToLower(k)
		if strings.Contains(lk, "token") ||
			strings.Contains(lk, "password") ||
			strings.Contains(lk, "secret") ||
			strings.Contains(lk, "photo") ||
			strings.Contains(lk, "image") ||
			strings.Contains(lk, "lat") ||
			strings.Contains(lk, "lng") ||
			strings.Contains(lk, "lon") ||
			strings.Contains(lk, "coord") {
			continue
		}
		key := truncate(k, 40)
		out[key] = redact(truncate(v, 400))
		n++
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
