// AP-099 研究员成长与纯虚拟伙伴 HTTP 处理器。
package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/repo"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GrowthHandler 服务端权威成长事件与伙伴档案。
type GrowthHandler struct {
	growth *repo.GrowthRepo
}

// NewGrowthHandler 构造。
func NewGrowthHandler(growth *repo.GrowthRepo) *GrowthHandler {
	return &GrowthHandler{growth: growth}
}

type growthEventRequest struct {
	EventID       string `json:"event_id" binding:"required"`
	Kind          string `json:"kind" binding:"required"`
	AnimalUUID    string `json:"animal_uuid"`
	SourceType    string `json:"source_type"`
	SourceID      string `json:"source_id"`
	Metadata      string `json:"metadata"`
	OverrideDelta *int64 `json:"override_delta"`
}

type growthResetRequest struct {
	Scope      string `json:"scope"` // researcher|companion|all
	AnimalUUID string `json:"animal_uuid"`
	Reason     string `json:"reason" binding:"required"`
	ToVersion  string `json:"to_version"`
}

// GetCatalog GET /api/v1/growth/catalog
func (h *GrowthHandler) GetCatalog(c *gin.Context) {
	cat := repo.Catalog()
	c.JSON(http.StatusOK, gin.H{
		"config_version":   cat.ConfigVersion,
		"tracks":           cat.Tracks,
		"events":           cat.Events,
		"companion_nodes":  cat.CompanionNodes,
		"rules":            cat.Rules,
		"level_thresholds": cat.LevelThresholds,
		"request_id":       middleware.GetRequestID(c),
	})
}

// GetResearcher GET /api/v1/growth/researcher
func (h *GrowthHandler) GetResearcher(c *gin.Context) {
	if h == nil || h.growth == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "growth unavailable", "reason_code": "db_unavailable"})
		return
	}
	deviceID := middleware.GetDeviceID(c)
	accountID := middleware.GetAccountID(c)
	tracks, err := h.growth.GetResearcher(accountID, deviceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list tracks failed", "reason_code": "growth_error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"owner_key":       repo.OwnerKey(accountID, deviceID),
		"config_version":  models.GrowthConfigVersion,
		"tracks":          tracks,
		"request_id":      middleware.GetRequestID(c),
	})
}

// PostEvent POST /api/v1/growth/events — 幂等成长事件。
func (h *GrowthHandler) PostEvent(c *gin.Context) {
	if h == nil || h.growth == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "growth unavailable", "reason_code": "db_unavailable"})
		return
	}
	var req growthEventRequest
	if err := middleware.BindStrictJSON(c, &req); err != nil {
		middleware.WriteBindError(c, err)
		return
	}
	deviceID := middleware.GetDeviceID(c)
	accountID := middleware.GetAccountID(c)
	res, err := h.growth.ApplyEvent(repo.ApplyGrowthRequest{
		DeviceID:      deviceID,
		AccountID:     accountID,
		EventID:       strings.TrimSpace(req.EventID),
		Kind:          strings.TrimSpace(req.Kind),
		AnimalUUID:    strings.TrimSpace(req.AnimalUUID),
		SourceType:    req.SourceType,
		SourceID:      req.SourceID,
		Metadata:      req.Metadata,
		OverrideDelta: req.OverrideDelta,
	})
	if err != nil {
		writeGrowthError(c, err)
		return
	}
	status := http.StatusCreated
	if res.Idempotent {
		status = http.StatusOK
	}
	c.JSON(status, gin.H{
		"event":            res.Event,
		"idempotent":       res.Idempotent,
		"researcher":       res.Researcher,
		"companion":        res.Companion,
		"nodes":            res.Nodes,
		"unlocked_nodes":   res.UnlockedNodes,
		"combat_unchanged": res.CombatUnchanged,
		"request_id":       middleware.GetRequestID(c),
	})
}

// ListEvents GET /api/v1/growth/events
func (h *GrowthHandler) ListEvents(c *gin.Context) {
	if h == nil || h.growth == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "growth unavailable", "reason_code": "db_unavailable"})
		return
	}
	deviceID := middleware.GetDeviceID(c)
	accountID := middleware.GetAccountID(c)
	afterID := uint(0)
	if v := c.Query("after_id"); v != "" {
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid after_id", "reason_code": "invalid_cursor"})
			return
		}
		afterID = uint(n)
	}
	limit := 50
	if v := c.Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil && n > 0 {
			limit = n
		}
	}
	rows, err := h.growth.ListEvents(accountID, deviceID, afterID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list events failed", "reason_code": "growth_error"})
		return
	}
	var nextAfter uint
	if len(rows) > 0 {
		nextAfter = rows[len(rows)-1].ID
	}
	c.JSON(http.StatusOK, gin.H{
		"events":     rows,
		"next_after": nextAfter,
		"request_id": middleware.GetRequestID(c),
	})
}

// ListCompanions GET /api/v1/growth/companions
func (h *GrowthHandler) ListCompanions(c *gin.Context) {
	if h == nil || h.growth == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "growth unavailable", "reason_code": "db_unavailable"})
		return
	}
	deviceID := middleware.GetDeviceID(c)
	accountID := middleware.GetAccountID(c)
	rows, err := h.growth.ListCompanions(accountID, deviceID, 100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list companions failed", "reason_code": "growth_error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"companions": rows,
		"request_id": middleware.GetRequestID(c),
	})
}

// GetCompanion GET /api/v1/growth/companions/:animal_uuid
func (h *GrowthHandler) GetCompanion(c *gin.Context) {
	if h == nil || h.growth == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "growth unavailable", "reason_code": "db_unavailable"})
		return
	}
	id := strings.TrimSpace(c.Param("animal_uuid"))
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid uuid", "reason_code": "invalid_uuid"})
		return
	}
	deviceID := middleware.GetDeviceID(c)
	accountID := middleware.GetAccountID(c)
	comp, nodes, err := h.growth.GetCompanion(accountID, deviceID, id)
	if err != nil {
		writeGrowthError(c, err)
		return
	}
	visible := 0
	unlocked := 0
	for _, n := range nodes {
		if n.Visible {
			visible++
		}
		if n.Unlocked {
			unlocked++
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"companion":       comp,
		"nodes":           nodes,
		"visible_nodes":   visible,
		"unlocked_nodes":  unlocked,
		"min_visible":     3,
		"combat_unchanged": true,
		"request_id":      middleware.GetRequestID(c),
	})
}

// Reset POST /api/v1/growth/reset — 可审计重置/迁移。
func (h *GrowthHandler) Reset(c *gin.Context) {
	if h == nil || h.growth == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "growth unavailable", "reason_code": "db_unavailable"})
		return
	}
	var req growthResetRequest
	if err := middleware.BindStrictJSON(c, &req); err != nil {
		middleware.WriteBindError(c, err)
		return
	}
	deviceID := middleware.GetDeviceID(c)
	accountID := middleware.GetAccountID(c)
	audit, err := h.growth.Reset(repo.ResetRequest{
		DeviceID:   deviceID,
		AccountID:  accountID,
		Scope:      repo.ResetScope(strings.TrimSpace(req.Scope)),
		AnimalUUID: strings.TrimSpace(req.AnimalUUID),
		Reason:     strings.TrimSpace(req.Reason),
		ToVersion:  strings.TrimSpace(req.ToVersion),
	})
	if err != nil {
		writeGrowthError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"audit":      audit,
		"request_id": middleware.GetRequestID(c),
	})
}

func writeGrowthError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repo.ErrGrowthInvalidEvent):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "reason_code": "growth_invalid_event"})
	case errors.Is(err, repo.ErrGrowthForbiddenKind):
		c.JSON(http.StatusBadRequest, gin.H{"error": "event kind not allowed", "reason_code": "growth_forbidden_kind"})
	case errors.Is(err, repo.ErrGrowthPaidPower):
		c.JSON(http.StatusForbidden, gin.H{"error": "paid power path is forbidden", "reason_code": "growth_paid_power_forbidden"})
	case errors.Is(err, repo.ErrGrowthDecayForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": "decay/real-world feeding is forbidden", "reason_code": "growth_decay_forbidden"})
	case errors.Is(err, repo.ErrGrowthNotFound), errors.Is(err, repo.ErrGrowthAnimalNotOwned):
		// 不泄露存在性
		c.JSON(http.StatusNotFound, gin.H{"error": "animal not found", "reason_code": "not_found"})
	case errors.Is(err, repo.ErrGrowthRepoUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "growth unavailable", "reason_code": "db_unavailable"})
	default:
		// 可能是 reason required 等
		msg := err.Error()
		if strings.Contains(msg, "reason") || strings.Contains(msg, "invalid") {
			c.JSON(http.StatusBadRequest, gin.H{"error": msg, "reason_code": "growth_invalid_event"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "growth error", "reason_code": "growth_error"})
	}
}
