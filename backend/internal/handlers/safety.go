// Package handlers — safety moderation report + account defaults (AP-056).
package handlers

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/safety"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SafetyHandler handles moderation reports and account safety defaults.
type SafetyHandler struct {
	db                 *gorm.DB
	strictMinorDefault bool
}

// NewSafetyHandler constructs SafetyHandler.
func NewSafetyHandler(db *gorm.DB, strictMinorDefault bool) *SafetyHandler {
	return &SafetyHandler{db: db, strictMinorDefault: strictMinorDefault}
}

type safetyReportRequest struct {
	Category    string `json:"category" binding:"required"` // abuse|injured|portrait|sensitive|other
	InferenceID string `json:"inference_id"`
	Note        string `json:"note"`
	// DecisionCode optional client hint; server re-derives public code.
	DecisionCode string `json:"decision_code"`
}

var allowedReportCategories = map[string]struct{}{
	"abuse":     {},
	"injured":   {},
	"portrait":  {},
	"sensitive": {},
	"other":     {},
}

// Report POST /api/v1/safety/report — abuse / injured animal report path.
// Never accepts image payloads; only structured metadata.
func (h *SafetyHandler) Report(c *gin.Context) {
	var req safetyReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload", "reason_code": "bad_request"})
		return
	}
	cat := strings.ToLower(strings.TrimSpace(req.Category))
	if _, ok := allowedReportCategories[cat]; !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid category", "reason_code": "bad_category"})
		return
	}
	if len(req.Note) > 1000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "note too long", "reason_code": "note_too_long"})
		return
	}
	// Reject accidental image/base64 dumps in note.
	if looksLikeImagePayload(req.Note) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "images not accepted", "reason_code": "image_not_allowed"})
		return
	}

	deviceID := middleware.GetDeviceID(c)
	decision := mapReportDecision(cat, req.DecisionCode)

	report := models.ModerationReport{
		ReportID:     uuid.NewString(),
		DeviceID:     deviceID,
		Category:     cat,
		DecisionCode: decision,
		InferenceID:  strings.TrimSpace(req.InferenceID),
		Note:         strings.TrimSpace(req.Note),
		Status:       "open",
	}
	if h.db != nil {
		if err := h.db.Create(&report).Error; err != nil {
			slog.Error("safety report save failed", "device_id", deviceID, "err", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "save failed", "reason_code": "db_error"})
			return
		}
	}

	// Audit without image / model internals.
	slog.Info("safety_report",
		"report_id", report.ReportID,
		"device_id", deviceID,
		"category", cat,
		"decision_code", decision,
		"inference_id", report.InferenceID,
		// note length only
		"note_len", len(report.Note),
	)

	c.JSON(http.StatusAccepted, gin.H{
		"status":        "accepted",
		"report_id":     report.ReportID,
		"decision_code": decision,
		"category":      cat,
		"request_id":    middleware.GetRequestID(c),
	})
}

// AccountDefaults GET /api/v1/account/defaults?minor=1
func (h *SafetyHandler) AccountDefaults(c *gin.Context) {
	isMinor := false
	switch strings.ToLower(c.Query("minor")) {
	case "1", "true", "yes":
		isMinor = true
	}
	d := safety.ResolveAccountDefaults(isMinor, h.strictMinorDefault)
	c.JSON(http.StatusOK, gin.H{
		"defaults":    d,
		"config":      gin.H{"strict_minor_defaults": h.strictMinorDefault},
		"request_id":  middleware.GetRequestID(c),
		"server_time": time.Now().UTC().Format(time.RFC3339),
	})
}

func mapReportDecision(category, clientCode string) string {
	// Prefer server mapping from category; ignore opaque client codes that look internal.
	switch category {
	case "abuse":
		return safety.CodeFlagAbuse
	case "injured":
		return safety.CodeFlagInjured
	case "portrait":
		return safety.CodeRejectPortrait
	case "sensitive":
		return safety.CodeFlagSensitive
	default:
		code := strings.TrimSpace(clientCode)
		switch code {
		case safety.CodeOK, safety.CodeRejectPortrait, safety.CodeRejectChildFocus,
			safety.CodeRejectSensitive, safety.CodeRejectUnsafe,
			safety.CodeFlagSensitive, safety.CodeFlagAbuse, safety.CodeFlagInjured:
			return code
		default:
			return safety.CodeRejectUnsafe
		}
	}
}

func looksLikeImagePayload(s string) bool {
	if len(s) > 200 && (strings.Contains(s, "data:image") || strings.Contains(s, "base64,")) {
		return true
	}
	// large binary-ish blobs
	if len(s) > 4000 {
		return true
	}
	return false
}
