package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/notification"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// NotificationHandler serves AP-084 inbox/outbox/prefs APIs.
type NotificationHandler struct {
	svc *notification.Service
}

// NewNotificationHandler constructs handler; db required.
func NewNotificationHandler(db *gorm.DB) *NotificationHandler {
	return &NotificationHandler{svc: notification.NewService(db, nil)}
}

// NewNotificationHandlerWithService for tests.
func NewNotificationHandlerWithService(svc *notification.Service) *NotificationHandler {
	return &NotificationHandler{svc: svc}
}

func notifOwner(c *gin.Context) string {
	if v, ok := c.Get("account_id"); ok {
		if s, ok := v.(string); ok && s != "" {
			return "acc:" + s
		}
	}
	if v, ok := c.Get("device_id"); ok {
		if s, ok := v.(string); ok && s != "" {
			return "dev:" + s
		}
	}
	return ""
}

// ListInbox GET /api/v1/notifications/inbox?cursor=&limit=
func (h *NotificationHandler) ListInbox(c *gin.Context) {
	owner := notifOwner(c)
	if owner == "" {
		middleware.WriteError(c, http.StatusUnauthorized, "unauthorized", "missing owner", false, nil)
		return
	}
	var after uint
	if cur := c.Query("cursor"); cur != "" {
		n, _ := strconv.ParseUint(cur, 10, 64)
		after = uint(n)
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	rows, err := h.svc.ListInbox(owner, after, limit)
	if err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "inbox_list_failed", err.Error(), true, nil)
		return
	}
	var next string
	if len(rows) > 0 {
		next = strconv.FormatUint(uint64(rows[len(rows)-1].ID), 10)
	}
	c.JSON(http.StatusOK, gin.H{"items": rows, "next_cursor": next, "request_id": middleware.GetRequestID(c)})
}

// ReadMessage POST /api/v1/notifications/inbox/:id/read
func (h *NotificationHandler) ReadMessage(c *gin.Context) {
	owner := notifOwner(c)
	id64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.MarkRead(owner, uint(id64)); err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "inbox_read_failed", err.Error(), true, nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "request_id": middleware.GetRequestID(c)})
}

// AckMessage POST /api/v1/notifications/inbox/:id/ack
func (h *NotificationHandler) AckMessage(c *gin.Context) {
	owner := notifOwner(c)
	id64, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.Ack(owner, uint(id64)); err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "inbox_ack_failed", err.Error(), true, nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "request_id": middleware.GetRequestID(c)})
}

// RegisterPushToken POST /api/v1/notifications/push-tokens
func (h *NotificationHandler) RegisterPushToken(c *gin.Context) {
	owner := notifOwner(c)
	var body struct {
		Token    string `json:"token"`
		Platform string `json:"platform"`
	}
	if err := middleware.BindStrictJSON(c, &body); err != nil {
		middleware.WriteBindError(c, err)
		return
	}
	if err := h.svc.UpsertPushToken(owner, strings.TrimSpace(body.Token), body.Platform); err != nil {
		middleware.WriteError(c, http.StatusBadRequest, "push_token_invalid", err.Error(), false, nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "request_id": middleware.GetRequestID(c)})
}

// DeletePushToken DELETE /api/v1/notifications/push-tokens
func (h *NotificationHandler) DeletePushToken(c *gin.Context) {
	var body struct {
		Token string `json:"token"`
	}
	if err := middleware.BindStrictJSON(c, &body); err != nil {
		middleware.WriteBindError(c, err)
		return
	}
	if err := h.svc.DisablePushToken(strings.TrimSpace(body.Token)); err != nil {
		middleware.WriteError(c, http.StatusBadRequest, "push_token_disable_failed", err.Error(), false, nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "request_id": middleware.GetRequestID(c)})
}

// GetPreferences GET /api/v1/notifications/preferences
func (h *NotificationHandler) GetPreferences(c *gin.Context) {
	owner := notifOwner(c)
	p, err := h.svc.GetPrefs(owner)
	if err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "prefs_failed", err.Error(), true, nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"preferences": p, "request_id": middleware.GetRequestID(c)})
}

// PutPreferences PUT /api/v1/notifications/preferences
func (h *NotificationHandler) PutPreferences(c *gin.Context) {
	owner := notifOwner(c)
	var body struct {
		MarketingConsent *bool   `json:"marketing_consent"`
		Minor            *bool   `json:"minor"`
		QuietStartHour   *int    `json:"quiet_start_hour"`
		QuietEndHour     *int    `json:"quiet_end_hour"`
		Timezone         *string `json:"timezone"`
	}
	if err := middleware.BindStrictJSON(c, &body); err != nil {
		middleware.WriteBindError(c, err)
		return
	}
	p, err := h.svc.UpdatePrefs(owner, body.MarketingConsent, body.Minor, body.QuietStartHour, body.QuietEndHour, body.Timezone)
	if err != nil {
		middleware.WriteError(c, http.StatusBadRequest, "prefs_invalid", err.Error(), false, nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"preferences": p, "request_id": middleware.GetRequestID(c)})
}

// EnqueueTest POST /api/v1/ops/notifications/enqueue — ops/test helper also usable by internal services.
func (h *NotificationHandler) EnqueueTest(c *gin.Context) {
	var body struct {
		OwnerKey  string `json:"owner_key"`
		Category  string `json:"category"`
		Title     string `json:"title"`
		Body      string `json:"body"`
		DedupeKey string `json:"dedupe_key"`
		Sensitive bool   `json:"sensitive"`
	}
	if err := middleware.BindStrictJSON(c, &body); err != nil {
		middleware.WriteBindError(c, err)
		return
	}
	if body.OwnerKey == "" {
		body.OwnerKey = notifOwner(c)
	}
	msg, err := h.svc.Enqueue(body.OwnerKey, body.Category, body.Title, body.Body, body.DedupeKey, body.Sensitive)
	if err != nil {
		middleware.WriteError(c, http.StatusBadRequest, "enqueue_failed", err.Error(), false, nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": msg, "request_id": middleware.GetRequestID(c)})
}

// ProcessOutbox POST /api/v1/ops/notifications/process — drain outbox (worker).
func (h *NotificationHandler) ProcessOutbox(c *gin.Context) {
	n, err := h.svc.ProcessOutbox(50)
	if err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "outbox_process_failed", err.Error(), true, nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"processed": n, "request_id": middleware.GetRequestID(c)})
}
