package handlers

import (
	"net/http"
	"strings"

	"animalpoke/backend/internal/liveops"
	"animalpoke/backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

// LiveOpsHandler serves AP-081 season/event instance APIs.
type LiveOpsHandler struct {
	store    *liveops.Store
	opsToken string
}

// NewLiveOpsHandler constructs a handler.
func NewLiveOpsHandler(store *liveops.Store, opsToken string) *LiveOpsHandler {
	if store == nil {
		store = liveops.Default()
	}
	return &LiveOpsHandler{store: store, opsToken: opsToken}
}

func (h *LiveOpsHandler) opsOK(c *gin.Context) bool {
	tok := strings.TrimSpace(c.GetHeader("X-AP-Ops-Token"))
	return h.opsToken != "" && tok == h.opsToken
}

func playerID(c *gin.Context) string {
	if v, ok := c.Get("device_id"); ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	if v, ok := c.Get("account_id"); ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return "anonymous"
}

// ListInstances GET /api/v1/liveops/instances
func (h *LiveOpsHandler) ListInstances(c *gin.Context) {
	list := h.store.ListInstances()
	c.JSON(http.StatusOK, gin.H{"instances": list, "server_time": middleware.GetRequestID(c), "request_id": middleware.GetRequestID(c)})
}

// GetInstance GET /api/v1/liveops/instances/:id
func (h *LiveOpsHandler) GetInstance(c *gin.Context) {
	inst, err := h.store.GetInstance(c.Param("id"))
	if err != nil {
		middleware.WriteError(c, http.StatusNotFound, "instance_not_found", err.Error(), false, nil)
		return
	}
	prog := h.store.GetProgress(inst.InstanceID, playerID(c))
	c.JSON(http.StatusOK, gin.H{"instance": inst, "progress": prog, "request_id": middleware.GetRequestID(c)})
}

// Enroll POST /api/v1/liveops/instances/:id/enroll
func (h *LiveOpsHandler) Enroll(c *gin.Context) {
	p, err := h.store.Enroll(c.Param("id"), playerID(c))
	if err != nil {
		middleware.WriteError(c, http.StatusConflict, "enroll_rejected", err.Error(), false, nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"progress": p, "request_id": middleware.GetRequestID(c)})
}

// Progress POST /api/v1/liveops/instances/:id/progress
func (h *LiveOpsHandler) Progress(c *gin.Context) {
	var body struct {
		Delta int `json:"delta"`
	}
	if err := middleware.BindStrictJSON(c, &body); err != nil {
		middleware.WriteBindError(c, err)
		return
	}
	p, err := h.store.AddProgress(c.Param("id"), playerID(c), body.Delta)
	if err != nil {
		middleware.WriteError(c, http.StatusConflict, "progress_rejected", err.Error(), false, nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"progress": p, "request_id": middleware.GetRequestID(c)})
}

// Claim POST /api/v1/liveops/instances/:id/claim
func (h *LiveOpsHandler) Claim(c *gin.Context) {
	var body struct {
		ClaimKey string `json:"claim_key"`
	}
	_ = middleware.BindStrictJSON(c, &body) // optional body
	p, ref, err := h.store.ClaimReward(c.Param("id"), playerID(c), body.ClaimKey)
	if err != nil {
		middleware.WriteError(c, http.StatusConflict, "claim_rejected", err.Error(), false, nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"progress": p, "reward_ref": ref, "request_id": middleware.GetRequestID(c)})
}

// Compensate POST /api/v1/liveops/instances/:id/compensate
func (h *LiveOpsHandler) Compensate(c *gin.Context) {
	p, ref, err := h.store.Compensate(c.Param("id"), playerID(c))
	if err != nil {
		middleware.WriteError(c, http.StatusConflict, "compensate_rejected", err.Error(), false, nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"progress": p, "compensation_ref": ref, "request_id": middleware.GetRequestID(c)})
}

// UpsertDefinition PUT /api/v1/ops/liveops/definitions
func (h *LiveOpsHandler) UpsertDefinition(c *gin.Context) {
	if !h.opsOK(c) {
		middleware.WriteError(c, http.StatusForbidden, "ops_forbidden", "ops access denied", false, nil)
		return
	}
	var def liveops.Definition
	if err := middleware.BindStrictJSON(c, &def); err != nil {
		middleware.WriteBindError(c, err)
		return
	}
	inst, err := h.store.UpsertDefinition(def)
	if err != nil {
		middleware.WriteError(c, http.StatusBadRequest, "definition_invalid", err.Error(), false, nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"instance": inst, "request_id": middleware.GetRequestID(c)})
}

// CancelInstance POST /api/v1/ops/liveops/instances/:id/cancel
func (h *LiveOpsHandler) CancelInstance(c *gin.Context) {
	if !h.opsOK(c) {
		middleware.WriteError(c, http.StatusForbidden, "ops_forbidden", "ops access denied", false, nil)
		return
	}
	inst, err := h.store.Cancel(c.Param("id"))
	if err != nil {
		middleware.WriteError(c, http.StatusConflict, "cancel_rejected", err.Error(), false, nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"instance": inst, "request_id": middleware.GetRequestID(c)})
}

// SettleInstance POST /api/v1/ops/liveops/instances/:id/settle
func (h *LiveOpsHandler) SettleInstance(c *gin.Context) {
	if !h.opsOK(c) {
		middleware.WriteError(c, http.StatusForbidden, "ops_forbidden", "ops access denied", false, nil)
		return
	}
	var body struct {
		BatchSize int `json:"batch_size"`
	}
	_ = middleware.BindStrictJSON(c, &body)
	inst, err := h.store.SettleBatch(c.Param("id"), body.BatchSize)
	if err != nil {
		middleware.WriteError(c, http.StatusConflict, "settle_rejected", err.Error(), false, nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"instance": inst, "request_id": middleware.GetRequestID(c)})
}
