// AP-096 数据驱动任务 HTTP 处理器。
package handlers

import (
	"errors"
	"net/http"
	"strings"

	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/questcatalog"
	"animalpoke/backend/internal/repo"

	"github.com/gin-gonic/gin"
)

// QuestHandler 服务端权威任务进度与领取。
type QuestHandler struct {
	quests *repo.QuestRepo
}

// NewQuestHandler 构造。
func NewQuestHandler(quests *repo.QuestRepo) *QuestHandler {
	return &QuestHandler{quests: quests}
}

type questEventRequest struct {
	EventID   string `json:"event_id" binding:"required"`
	EventType string `json:"event_type" binding:"required"`
	Delta     int64  `json:"delta"`
	Payload   string `json:"payload"`
}

// ListQuests GET /api/v1/quests
func (h *QuestHandler) ListQuests(c *gin.Context) {
	if h == nil || h.quests == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "quest unavailable", "reason_code": "db_unavailable"})
		return
	}
	freeOnly := c.Query("free") == "1" || strings.EqualFold(c.Query("free"), "true")
	deviceID := middleware.GetDeviceID(c)
	accountID := middleware.GetAccountID(c)
	views, err := h.quests.ListForOwner(accountID, deviceID, freeOnly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list quests failed", "reason_code": "quest_error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"quests":         views,
		"config_version": questcatalog.ConfigVersion,
		"request_id":     middleware.GetRequestID(c),
	})
}

// GetQuest GET /api/v1/quests/:quest_id
func (h *QuestHandler) GetQuest(c *gin.Context) {
	if h == nil || h.quests == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "quest unavailable", "reason_code": "db_unavailable"})
		return
	}
	questID := strings.TrimSpace(c.Param("quest_id"))
	if questID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "quest_id required", "reason_code": "bad_request"})
		return
	}
	deviceID := middleware.GetDeviceID(c)
	accountID := middleware.GetAccountID(c)
	views, err := h.quests.ListForOwner(accountID, deviceID, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "get quest failed", "reason_code": "quest_error"})
		return
	}
	for _, v := range views {
		if v.Definition.QuestID == questID {
			c.JSON(http.StatusOK, gin.H{"quest": v, "request_id": middleware.GetRequestID(c)})
			return
		}
	}
	// 可能未启用或不在窗口
	def, err := h.quests.GetDefinition(questID)
	if err != nil {
		if errors.Is(err, repo.ErrQuestNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "quest not found", "reason_code": "quest_not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "get quest failed", "reason_code": "quest_error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"quest": gin.H{
			"definition": def,
			"progress":   nil,
			"claimable":  false,
			"free":       def.Free,
		},
		"request_id": middleware.GetRequestID(c),
	})
}

// Catalog GET /api/v1/quests/catalog
func (h *QuestHandler) Catalog(c *gin.Context) {
	if h == nil || h.quests == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "quest unavailable", "reason_code": "db_unavailable"})
		return
	}
	defs, err := h.quests.ListDefinitions(false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "catalog failed", "reason_code": "quest_error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"definitions":    defs,
		"config_version": questcatalog.ConfigVersion,
		"count":          len(defs),
		"request_id":     middleware.GetRequestID(c),
	})
}

// ApplyEvents POST /api/v1/quests/events
// 仅接受可信业务事件；打开页面类事件返回 400 quest_event_forbidden。
func (h *QuestHandler) ApplyEvents(c *gin.Context) {
	if h == nil || h.quests == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "quest unavailable", "reason_code": "db_unavailable"})
		return
	}
	var req questEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "event_id, event_type required", "reason_code": "bad_request"})
		return
	}
	deviceID := middleware.GetDeviceID(c)
	accountID := middleware.GetAccountID(c)
	res, err := h.quests.ApplyEvent(repo.ApplyEventRequest{
		DeviceID:  deviceID,
		AccountID: accountID,
		EventID:   req.EventID,
		EventType: req.EventType,
		Delta:     req.Delta,
		Payload:   req.Payload,
	})
	if err != nil {
		writeQuestError(c, err)
		return
	}
	status := http.StatusOK
	if !res.Idempotent {
		status = http.StatusCreated
	}
	c.JSON(status, gin.H{
		"idempotent":        res.Idempotent,
		"event_type":        res.EventType,
		"updated_quest_ids": res.Updated,
		"request_id":        middleware.GetRequestID(c),
	})
}

// Claim POST /api/v1/quests/:quest_id/claim
func (h *QuestHandler) Claim(c *gin.Context) {
	if h == nil || h.quests == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "quest unavailable", "reason_code": "db_unavailable"})
		return
	}
	questID := strings.TrimSpace(c.Param("quest_id"))
	if questID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "quest_id required", "reason_code": "bad_request"})
		return
	}
	deviceID := middleware.GetDeviceID(c)
	accountID := middleware.GetAccountID(c)
	res, err := h.quests.Claim(repo.ClaimRequest{
		DeviceID:  deviceID,
		AccountID: accountID,
		QuestID:   questID,
	})
	if err != nil {
		writeQuestError(c, err)
		return
	}
	status := http.StatusCreated
	if res.Idempotent {
		status = http.StatusOK
	}
	c.JSON(status, gin.H{
		"claim":      res.Claim,
		"idempotent": res.Idempotent,
		"gold":       res.Gold,
		"stamina":    res.Stamina,
		"balances":   res.Balances,
		"request_id": middleware.GetRequestID(c),
	})
}

// Compensate POST /api/v1/quests/compensate
func (h *QuestHandler) Compensate(c *gin.Context) {
	if h == nil || h.quests == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "quest unavailable", "reason_code": "db_unavailable"})
		return
	}
	deviceID := middleware.GetDeviceID(c)
	accountID := middleware.GetAccountID(c)
	res, err := h.quests.CompensateExpired(accountID, deviceID)
	if err != nil {
		writeQuestError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"compensated": res.Compensated,
		"expired":     res.Expired,
		"request_id":  middleware.GetRequestID(c),
	})
}

func writeQuestError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repo.ErrQuestEventForbidden):
		c.JSON(http.StatusBadRequest, gin.H{
			"error":       "event type forbidden (cannot fake by reopening page)",
			"reason_code": "quest_event_forbidden",
		})
	case errors.Is(err, repo.ErrQuestEventUntrusted):
		c.JSON(http.StatusBadRequest, gin.H{
			"error":       "event type not trusted",
			"reason_code": "quest_event_untrusted",
		})
	case errors.Is(err, repo.ErrQuestEventInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid event", "reason_code": "quest_event_invalid"})
	case errors.Is(err, repo.ErrQuestNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "quest not found", "reason_code": "quest_not_found"})
	case errors.Is(err, repo.ErrQuestNotClaimable):
		c.JSON(http.StatusConflict, gin.H{"error": "quest not claimable", "reason_code": "quest_not_claimable"})
	case errors.Is(err, repo.ErrQuestExpired):
		c.JSON(http.StatusConflict, gin.H{"error": "quest expired; use compensate", "reason_code": "quest_expired"})
	case errors.Is(err, repo.ErrQuestPrereq):
		c.JSON(http.StatusConflict, gin.H{"error": "prerequisites unmet", "reason_code": "quest_prereq_unmet"})
	case errors.Is(err, repo.ErrQuestDisabled):
		c.JSON(http.StatusConflict, gin.H{"error": "quest disabled", "reason_code": "quest_disabled"})
	case errors.Is(err, repo.ErrInvalidOwner):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid owner", "reason_code": "bad_request"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "quest error", "reason_code": "quest_error"})
	}
}
