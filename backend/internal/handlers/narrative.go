package handlers

import (
	"net/http"
	"strings"

	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/narrativecatalog"
	"animalpoke/backend/internal/repo"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// NarrativeHandler 剧情进度 API（AP-132）。
type NarrativeHandler struct {
	repo *repo.NarrativeRepo
}

// NewNarrativeHandler 构造。
func NewNarrativeHandler(r *repo.NarrativeRepo) *NarrativeHandler {
	return &NarrativeHandler{repo: r}
}

func (h *NarrativeHandler) owner(c *gin.Context) (ownerKey, deviceID, accountID string, ok bool) {
	deviceID = middleware.GetDeviceID(c)
	if deviceID == "" {
		middleware.WriteError(c, http.StatusUnauthorized, "unauthorized", "unauthorized", false, nil)
		return "", "", "", false
	}
	accountID = strings.TrimSpace(middleware.GetAccountID(c))
	ownerKey = repo.OwnerKey(accountID, deviceID)
	return ownerKey, deviceID, accountID, true
}

// GetCatalog GET /narrative/catalog
func (h *NarrativeHandler) GetCatalog(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"content_version": narrativecatalog.ContentVersion,
		"chapters":        []string{"ch1", "ch3", "ch4", "fail_forward"},
		"source":          "server",
		"request_id":      middleware.GetRequestID(c),
	})
}

// GetNode GET /narrative/nodes/:node_id
func (h *NarrativeHandler) GetNode(c *gin.Context) {
	n, err := h.repo.GetNode(c.Param("node_id"))
	if err != nil {
		writeNarrativeErr(c, err)
		return
	}
	choices, _ := h.repo.ListChoices(n.NodeID)
	items := make([]gin.H, 0, len(choices))
	for _, ch := range choices {
		items = append(items, gin.H{
			"choice_id": ch.ChoiceID, "label": ch.Label, "prompt": ch.Prompt,
			"to_node_id": ch.ToNodeID, "sort_order": ch.SortOrder,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"node": gin.H{
			"node_id": n.NodeID, "chapter_id": n.ChapterID, "title": n.Title, "body": n.Body,
			"kind": n.Kind, "content_version": n.ContentVersion,
			"layer": "authored_canon", // 与虚构花絮区分
		},
		"choices":    items,
		"request_id": middleware.GetRequestID(c),
	})
}

// GetProgress GET /narrative/progress?chapter=
func (h *NarrativeHandler) GetProgress(c *gin.Context) {
	ownerKey, deviceID, accountID, ok := h.owner(c)
	if !ok {
		return
	}
	chapter := strings.TrimSpace(c.DefaultQuery("chapter", "ch1"))
	p, err := h.repo.EnsureProgress(ownerKey, deviceID, accountID, chapter)
	if err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "narrative_failed", "progress failed", true, nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"progress": gin.H{
			"chapter_id": p.ChapterID, "current_node_id": p.CurrentNodeID,
			"checkpoint_node": p.CheckpointNode, "flags_json": p.FlagsJSON,
			"relationships_json": p.RelationshipsJSON, "server_version": p.ServerVersion,
			"content_version": p.ContentVersion, "last_choice_id": p.LastChoiceID,
		},
		"request_id": middleware.GetRequestID(c),
	})
}

// PullAll GET /narrative/progress/all
func (h *NarrativeHandler) PullAll(c *gin.Context) {
	ownerKey, _, _, ok := h.owner(c)
	if !ok {
		return
	}
	rows, err := h.repo.PullProgress(ownerKey)
	if err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "narrative_failed", "pull failed", true, nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows, "request_id": middleware.GetRequestID(c)})
}

// SubmitChoice POST /narrative/choices
func (h *NarrativeHandler) SubmitChoice(c *gin.Context) {
	ownerKey, deviceID, accountID, ok := h.owner(c)
	if !ok {
		return
	}
	var req struct {
		ChapterID   string `json:"chapter_id" binding:"required"`
		ChoiceID    string `json:"choice_id" binding:"required"`
		OperationID string `json:"operation_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, http.StatusBadRequest, "bad_request", "invalid body", false, nil)
		return
	}
	p, idem, err := h.repo.SubmitChoice(ownerKey, deviceID, accountID, req.ChapterID, req.ChoiceID, req.OperationID)
	if err != nil {
		writeNarrativeErr(c, err)
		return
	}
	// clue side effects from flags
	if strings.Contains(p.FlagsJSON, "clue:") {
		_ = h.repo.UpsertClue(ownerKey, "rain_shadow", clueStatusFromFlags(p.FlagsJSON), "player_choice", "chapter3 judgment")
	}
	status := http.StatusCreated
	if idem {
		status = http.StatusOK
	}
	c.JSON(status, gin.H{
		"progress": gin.H{
			"chapter_id": p.ChapterID, "current_node_id": p.CurrentNodeID,
			"server_version": p.ServerVersion, "flags_json": p.FlagsJSON,
			"relationships_json": p.RelationshipsJSON,
		},
		"idempotent": idem,
		"request_id": middleware.GetRequestID(c),
	})
}

// MarkSeen POST /narrative/seen
func (h *NarrativeHandler) MarkSeen(c *gin.Context) {
	ownerKey, deviceID, accountID, ok := h.owner(c)
	if !ok {
		return
	}
	var req struct {
		NodeID  string `json:"node_id" binding:"required"`
		Summary string `json:"summary"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, http.StatusBadRequest, "bad_request", "invalid body", false, nil)
		return
	}
	if err := h.repo.MarkSeen(ownerKey, deviceID, accountID, req.NodeID, req.Summary); err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "narrative_failed", "seen failed", true, nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "request_id": middleware.GetRequestID(c)})
}

// ListSeen GET /narrative/seen
func (h *NarrativeHandler) ListSeen(c *gin.Context) {
	ownerKey, _, _, ok := h.owner(c)
	if !ok {
		return
	}
	rows, err := h.repo.ListSeen(ownerKey)
	if err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "narrative_failed", "list failed", true, nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows, "request_id": middleware.GetRequestID(c)})
}

// FailForward POST /narrative/fail-forward
func (h *NarrativeHandler) FailForward(c *gin.Context) {
	ownerKey, deviceID, accountID, ok := h.owner(c)
	if !ok {
		return
	}
	var req struct {
		MissCount int    `json:"miss_count"`
		Reason    string `json:"reason"` // miss|weather|no_camera|permission|offline
	}
	_ = c.ShouldBindJSON(&req)
	if req.MissCount <= 0 {
		req.MissCount = 1
	}
	n, err := h.repo.FailForward(ownerKey, deviceID, accountID, req.MissCount, req.Reason)
	if err != nil {
		writeNarrativeErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"node": gin.H{
			"node_id": n.NodeID, "title": n.Title, "body": n.Body, "kind": n.Kind,
			"layer": "authored_canon",
		},
		"request_id": middleware.GetRequestID(c),
	})
}

// ObservationEvent POST /narrative/observation — 捕获/观察后触发碎片（AP-118）。
func (h *NarrativeHandler) ObservationEvent(c *gin.Context) {
	ownerKey, deviceID, accountID, ok := h.owner(c)
	if !ok {
		return
	}
	var req struct {
		OperationID      string         `json:"operation_id" binding:"required"`
		Species          string         `json:"species"`
		IsFirstSpecies   bool           `json:"is_first_species"`
		ObservationCount int            `json:"observation_count"`
		Weather          string         `json:"weather"`
		SpeciesSeen      []string       `json:"species_seen"`
		Extra            map[string]any `json:"extra"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, http.StatusBadRequest, "bad_request", "invalid body", false, nil)
		return
	}
	ctx := map[string]any{
		"species":           req.Species,
		"first_species":     req.Species,
		"is_first_species":  req.IsFirstSpecies,
		"observation_count": req.ObservationCount,
		"weather":           req.Weather,
	}
	seen := map[string]bool{}
	for _, s := range req.SpeciesSeen {
		seen[s] = true
	}
	if req.Species != "" {
		seen[req.Species] = true
	}
	ctx["species_seen"] = seen
	for k, v := range req.Extra {
		ctx[k] = v
	}
	frags, err := h.repo.TryUnlockFragments(ownerKey, deviceID, accountID, req.OperationID, ctx)
	if err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "narrative_failed", "fragment unlock failed", true, nil)
		return
	}
	items := make([]gin.H, 0, len(frags))
	for _, f := range frags {
		items = append(items, gin.H{
			"fragment_id": f.FragmentID, "title": f.Title, "body": f.Body,
			"why": "trigger_matched", "layer": "authored_canon",
		})
	}
	c.JSON(http.StatusOK, gin.H{"unlocked": items, "request_id": middleware.GetRequestID(c)})
}

// ListClues GET /narrative/clues
func (h *NarrativeHandler) ListClues(c *gin.Context) {
	ownerKey, _, _, ok := h.owner(c)
	if !ok {
		return
	}
	rows, err := h.repo.ListClues(ownerKey)
	if err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "narrative_failed", "clues failed", true, nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows, "request_id": middleware.GetRequestID(c)})
}

// UpdateClue POST /narrative/clues
func (h *NarrativeHandler) UpdateClue(c *gin.Context) {
	ownerKey, _, _, ok := h.owner(c)
	if !ok {
		return
	}
	var req struct {
		ClueID   string `json:"clue_id" binding:"required"`
		Status   string `json:"status" binding:"required"` // unknown|pending|confirmed|disputed
		Source   string `json:"source"`
		Evidence string `json:"evidence"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, http.StatusBadRequest, "bad_request", "invalid body", false, nil)
		return
	}
	st := strings.ToLower(strings.TrimSpace(req.Status))
	switch st {
	case "unknown", "pending", "confirmed", "disputed":
	default:
		middleware.WriteError(c, http.StatusBadRequest, "bad_request", "invalid status", false, nil)
		return
	}
	if err := h.repo.UpsertClue(ownerKey, req.ClueID, st, req.Source, req.Evidence); err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "narrative_failed", "clue update failed", true, nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "request_id": middleware.GetRequestID(c)})
}

func writeNarrativeErr(c *gin.Context, err error) {
	switch err {
	case repo.ErrNarrativeNotFound:
		middleware.WriteError(c, http.StatusNotFound, "narrative_not_found", "not found", false, nil)
	case repo.ErrNarrativeIllegal:
		middleware.WriteError(c, http.StatusConflict, "narrative_illegal", "illegal transition or forged flag", false, nil)
	case repo.ErrNarrativeDuplicate:
		middleware.WriteError(c, http.StatusConflict, "narrative_duplicate", "choice already submitted", false, nil)
	case repo.ErrNarrativeWithdrawn:
		middleware.WriteError(c, http.StatusGone, "narrative_withdrawn", "content withdrawn", false, nil)
	default:
		middleware.WriteError(c, http.StatusInternalServerError, "narrative_failed", "narrative error", true, nil)
	}
}

// EndingSummary GET /narrative/ending-summary — 选择因果摘要（AP-119 foldback）。
func (h *NarrativeHandler) EndingSummary(c *gin.Context) {
	ownerKey, _, _, ok := h.owner(c)
	if !ok {
		return
	}
	rows, err := h.repo.PullProgress(ownerKey)
	if err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "narrative_failed", "summary failed", true, nil)
		return
	}
	items := make([]gin.H, 0, len(rows))
	for _, p := range rows {
		items = append(items, gin.H{
			"chapter_id":         p.ChapterID,
			"current_node_id":    p.CurrentNodeID,
			"flags_json":         p.FlagsJSON,
			"relationships_json": p.RelationshipsJSON,
			"last_choice_id":     p.LastChoiceID,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"summary":    items,
		"note":       "foldback: local differences preserved in flags/relationships; mainline shared",
		"request_id": middleware.GetRequestID(c),
	})
}

func clueStatusFromFlags(flagsJSON string) string {
	if strings.Contains(flagsJSON, "hold") {
		return "pending"
	}
	if strings.Contains(flagsJSON, "tentative") || strings.Contains(flagsJSON, "disputed") {
		return "disputed"
	}
	return "unknown"
}

// NewOpID helper for tests.
func NewNarrativeOpID() string { return uuid.NewString() }
