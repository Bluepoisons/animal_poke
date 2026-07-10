package handlers

import (
	"log/slog"
	"net/http"
	"strings"
	"unicode/utf8"

	"animalpoke/backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

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
