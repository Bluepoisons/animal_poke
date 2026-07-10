package handlers

import (
	"log/slog"
	"net/http"
	"strings"
	"unicode/utf8"

	"animalpoke/backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

const maxErrorReportBytes = 16 * 1024

type errorReportRequest struct {
	Message   string            `json:"message"`
	Stack     string            `json:"stack"`
	Component string            `json:"component"`
	Route     string            `json:"route"`
	Release   string            `json:"release"`
	Level     string            `json:"level"`
	Extra     map[string]string `json:"extra"`
}

// ErrorReportHandler 接收前端错误（已鉴权），脱敏后记录。
type ErrorReportHandler struct{}

func NewErrorReportHandler() *ErrorReportHandler { return &ErrorReportHandler{} }

// Report POST /api/v1/errors/report
func (h *ErrorReportHandler) Report(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxErrorReportBytes)
	var req errorReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload", "reason_code": "bad_request", "request_id": middleware.GetRequestID(c)})
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "message required", "reason_code": "missing_message", "request_id": middleware.GetRequestID(c)})
		return
	}
	// 脱敏：截断、剔除疑似 token/key
	msg := redact(truncate(req.Message, 500))
	stack := redact(truncate(req.Stack, 2000))
	device := middleware.GetDeviceID(c)
	if len(device) > 8 {
		device = device[:4] + "…" + device[len(device)-4:]
	}
	slog.Warn("client_error",
		"request_id", middleware.GetRequestID(c),
		"device", device,
		"message", msg,
		"component", truncate(req.Component, 80),
		"route", truncate(req.Route, 120),
		"release", truncate(req.Release, 64),
		"level", truncate(req.Level, 16),
		"stack", stack,
	)
	c.JSON(http.StatusAccepted, gin.H{"accepted": true, "request_id": middleware.GetRequestID(c)})
}

func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n]) + "…"
}

func redact(s string) string {
	lower := strings.ToLower(s)
	for _, needle := range []string{"bearer ", "jwt", "api_key", "apikey", "password", "authorization"} {
		if strings.Contains(lower, needle) {
			return "[redacted]"
		}
	}
	return s
}
