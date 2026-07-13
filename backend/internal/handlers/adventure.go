package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/repo"
	"animalpoke/backend/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AdventureHandler generates one structured fictional expedition encounter.
type AdventureHandler struct {
	aiService *services.AIService
	animals   *repo.AnimalRepo
	growth    *repo.GrowthRepo
	runs      *repo.AdventureRepo
}

func NewAdventureHandler(aiService *services.AIService) *AdventureHandler {
	return &AdventureHandler{aiService: aiService}
}

func NewAdventureHandlerWithRepos(aiService *services.AIService, animals *repo.AnimalRepo, growth *repo.GrowthRepo, runs *repo.AdventureRepo) *AdventureHandler {
	return &AdventureHandler{aiService: aiService, animals: animals, growth: growth, runs: runs}
}

type adventureRequest struct {
	AnimalUUID  string `json:"animal_uuid" binding:"required"`
	Theme       string `json:"theme" binding:"required"`
	OperationID string `json:"operation_id" binding:"required"`
}

// Generate handles POST /adventures/generate.
func (h *AdventureHandler) Generate(c *gin.Context) {
	var req adventureRequest
	if err := middleware.BindStrictJSON(c, &req); err != nil {
		middleware.WriteBindError(c, err)
		return
	}
	if h.animals == nil {
		middleware.WriteError(c, http.StatusServiceUnavailable, "adventure_unavailable", "animal repository unavailable", true, nil)
		return
	}
	operationID := strings.TrimSpace(req.OperationID)
	if operationID == "" || len(operationID) > 128 {
		middleware.WriteError(c, http.StatusBadRequest, "invalid_adventure_operation", "operation_id is required", false, nil)
		return
	}
	if headerKey := strings.TrimSpace(c.GetHeader(middleware.HeaderIdempotencyKey)); headerKey != "" && headerKey != operationID {
		middleware.WriteError(c, http.StatusBadRequest, "invalid_adventure_operation", "operation_id must match Idempotency-Key", false, nil)
		return
	}
	deviceID := middleware.GetDeviceID(c)
	accountID := middleware.GetAccountID(c)
	if h.runs != nil {
		if existing, findErr := h.runs.FindByOperation(operationID, accountID, deviceID); findErr == nil {
			var restored services.AdventureResult
			if json.Unmarshal([]byte(existing.ResultJSON), &restored) == nil {
				c.JSON(http.StatusOK, publicAdventure(restored))
				return
			}
		} else if !errors.Is(findErr, repo.ErrAdventureNotFound) {
			writeAdventureError(c, findErr)
			return
		}
	}

	animalID := strings.TrimSpace(req.AnimalUUID)
	animal, err := h.animals.FindByUUID(animalID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			middleware.WriteError(c, http.StatusNotFound, "animal_not_found", "animal not found", false, nil)
			return
		}
		middleware.WriteError(c, http.StatusInternalServerError, "adventure_animal_lookup_failed", "animal lookup failed", true, nil)
		return
	}
	if !repo.OwnsAnimal(animal, deviceID, accountID) {
		middleware.WriteError(c, http.StatusNotFound, "animal_not_found", "animal not found", false, nil)
		return
	}

	bondLevel := 0
	if h.growth != nil {
		if companion, _, growthErr := h.growth.GetCompanion(accountID, deviceID, animalID); growthErr == nil && companion != nil {
			bondLevel = companion.BondLevel
		}
	}
	nickname := strings.TrimSpace(animal.Nickname)
	if nickname == "" {
		nickname = defaultAdventureNickname(animal.UUID)
	}
	input := services.AdventureInput{
		AnimalID:       animal.UUID,
		Nickname:       nickname,
		Species:        animal.Species,
		SpeciesLabelZH: animal.SpeciesLabelZH,
		Breed:          animal.Breed,
		Class:          animal.Class,
		Element:        animal.Element,
		HP:             animal.HP,
		ATK:            animal.ATK,
		DEF:            animal.DEF,
		SPD:            animal.SPD,
		BondLevel:      bondLevel,
		Theme:          req.Theme,
	}
	if err := input.Validate(); err != nil {
		middleware.WriteError(c, http.StatusBadRequest, "invalid_adventure_input", err.Error(), false, nil)
		return
	}
	result, err := h.aiService.GenerateAdventureContext(c.Request.Context(), input)
	if err != nil {
		WriteProviderError(c, err, "adventure generation failed")
		return
	}
	if h.runs == nil {
		middleware.WriteError(c, http.StatusServiceUnavailable, "adventure_unavailable", "adventure repository unavailable", true, nil)
		return
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "adventure_encode_failed", "adventure encode failed", true, nil)
		return
	}
	if err := h.runs.Create(&models.AdventureRun{
		RunID:         result.AdventureID,
		OwnerKey:      repo.OwnerKey(accountID, deviceID),
		OperationID:   operationID,
		DeviceID:      deviceID,
		AccountID:     accountID,
		AnimalUUID:    animal.UUID,
		Theme:         result.Theme,
		Title:         result.Title,
		Status:        "generated",
		ResultJSON:    string(resultJSON),
		PromptVersion: result.PromptVersion,
		Source:        result.Source,
		CreatedAt:     time.Now().UTC(),
	}); err != nil {
		if existing, findErr := h.runs.FindByOperation(operationID, accountID, deviceID); findErr == nil {
			var restored services.AdventureResult
			if json.Unmarshal([]byte(existing.ResultJSON), &restored) == nil {
				c.JSON(http.StatusOK, publicAdventure(restored))
				return
			}
		}
		middleware.WriteError(c, http.StatusInternalServerError, "adventure_persist_failed", "adventure save failed", true, nil)
		return
	}
	c.JSON(http.StatusCreated, publicAdventure(*result))
}

type adventureChoiceRequest struct {
	ChoiceID string `json:"choice_id" binding:"required"`
}

// CompleteChoice settles one generated choice and grants one idempotent memory event.
func (h *AdventureHandler) CompleteChoice(c *gin.Context) {
	if h.runs == nil || h.growth == nil {
		middleware.WriteError(c, http.StatusServiceUnavailable, "adventure_unavailable", "adventure settlement unavailable", true, nil)
		return
	}
	var req adventureChoiceRequest
	if err := middleware.BindStrictJSON(c, &req); err != nil {
		middleware.WriteBindError(c, err)
		return
	}
	choiceID := strings.TrimSpace(req.ChoiceID)
	deviceID := middleware.GetDeviceID(c)
	accountID := middleware.GetAccountID(c)
	run, err := h.runs.FindOwned(c.Param("run_id"), accountID, deviceID)
	if err != nil {
		writeAdventureError(c, err)
		return
	}
	var story services.AdventureResult
	if err := json.Unmarshal([]byte(run.ResultJSON), &story); err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "adventure_corrupt", "adventure data invalid", false, nil)
		return
	}
	var selected *services.AdventureChoice
	for i := range story.Choices {
		if story.Choices[i].ID == choiceID {
			selected = &story.Choices[i]
			break
		}
	}
	if selected == nil {
		middleware.WriteError(c, http.StatusBadRequest, "invalid_adventure_choice", "choice does not belong to adventure", false, nil)
		return
	}

	var completed *models.AdventureRun
	var settlement *repo.ApplyGrowthResult
	var idempotent bool
	growthFailed := false
	err = h.runs.Transaction(func(tx *gorm.DB) error {
		var txErr error
		completed, idempotent, txErr = h.runs.WithTx(tx).Complete(
			run.RunID,
			accountID,
			deviceID,
			selected.ID,
			selected.Outcome,
			story.Souvenir.Name,
			selected.BondDelta,
		)
		if txErr != nil {
			return txErr
		}
		settlement, txErr = h.growth.WithTx(tx).ApplyEvent(repo.ApplyGrowthRequest{
			DeviceID:   deviceID,
			AccountID:  accountID,
			EventID:    "adventure:" + run.RunID + ":complete",
			Kind:       models.GrowthEventCompanionMemory,
			AnimalUUID: run.AnimalUUID,
			SourceType: "adventure",
			SourceID:   run.RunID,
			Metadata:   `{"choice_id":"` + selected.ID + `"}`,
		})
		growthFailed = txErr != nil
		return txErr
	})
	if err != nil {
		if growthFailed {
			writeGrowthError(c, err)
		} else {
			writeAdventureError(c, err)
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"adventure_id":   completed.RunID,
		"status":         completed.Status,
		"choice":         selected,
		"outcome":        completed.Outcome,
		"souvenir":       story.Souvenir,
		"companion":      settlement.Companion,
		"nodes":          settlement.Nodes,
		"unlocked_nodes": settlement.UnlockedNodes,
		"idempotent":     idempotent || settlement.Idempotent,
		"request_id":     middleware.GetRequestID(c),
	})
}

// Get restores a generated or completed run after refresh.
func (h *AdventureHandler) Get(c *gin.Context) {
	if h.runs == nil {
		middleware.WriteError(c, http.StatusServiceUnavailable, "adventure_unavailable", "adventure repository unavailable", true, nil)
		return
	}
	run, err := h.runs.FindOwned(c.Param("run_id"), middleware.GetAccountID(c), middleware.GetDeviceID(c))
	if err != nil {
		writeAdventureError(c, err)
		return
	}
	var story services.AdventureResult
	if err := json.Unmarshal([]byte(run.ResultJSON), &story); err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "adventure_corrupt", "adventure data invalid", false, nil)
		return
	}
	response := gin.H{
		"story":              publicAdventure(story),
		"status":             run.Status,
		"selected_choice_id": run.SelectedChoiceID,
		"completed_at":       run.CompletedAt,
		"request_id":         middleware.GetRequestID(c),
	}
	if run.Status == "completed" {
		response["outcome"] = run.Outcome
		response["souvenir"] = story.Souvenir
		response["bond_delta"] = run.BondDelta
	}
	c.JSON(http.StatusOK, response)
}

// List returns recent adventure summaries, optionally scoped to one animal.
func (h *AdventureHandler) List(c *gin.Context) {
	if h.runs == nil {
		middleware.WriteError(c, http.StatusServiceUnavailable, "adventure_unavailable", "adventure repository unavailable", true, nil)
		return
	}
	limit := 12
	if raw := c.Query("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	rows, err := h.runs.ListOwned(middleware.GetAccountID(c), middleware.GetDeviceID(c), c.Query("animal_uuid"), limit)
	if err != nil {
		writeAdventureError(c, err)
		return
	}
	type summary struct {
		AdventureID string     `json:"adventure_id"`
		AnimalUUID  string     `json:"animal_uuid"`
		Theme       string     `json:"theme"`
		Title       string     `json:"title"`
		Status      string     `json:"status"`
		ChoiceID    string     `json:"choice_id,omitempty"`
		Outcome     string     `json:"outcome,omitempty"`
		Souvenir    string     `json:"souvenir,omitempty"`
		BondDelta   int        `json:"bond_delta"`
		CreatedAt   time.Time  `json:"created_at"`
		CompletedAt *time.Time `json:"completed_at,omitempty"`
	}
	items := make([]summary, 0, len(rows))
	for _, row := range rows {
		items = append(items, summary{
			AdventureID: row.RunID,
			AnimalUUID:  row.AnimalUUID,
			Theme:       row.Theme,
			Title:       row.Title,
			Status:      row.Status,
			ChoiceID:    row.SelectedChoiceID,
			Outcome:     row.Outcome,
			Souvenir:    row.SouvenirName,
			BondDelta:   row.BondDelta,
			CreatedAt:   row.CreatedAt,
			CompletedAt: row.CompletedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "request_id": middleware.GetRequestID(c)})
}

func writeAdventureError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repo.ErrAdventureNotFound):
		middleware.WriteError(c, http.StatusNotFound, "adventure_not_found", "adventure not found", false, nil)
	case errors.Is(err, repo.ErrAdventureChoiceConflict):
		middleware.WriteError(c, http.StatusConflict, "adventure_choice_conflict", "adventure already completed with another choice", false, nil)
	case errors.Is(err, repo.ErrAdventureRepoUnavailable):
		middleware.WriteError(c, http.StatusServiceUnavailable, "adventure_unavailable", "adventure repository unavailable", true, nil)
	default:
		middleware.WriteError(c, http.StatusInternalServerError, "adventure_error", "adventure operation failed", true, nil)
	}
}

func defaultAdventureNickname(animalID string) string {
	names := []string{"Milo", "Luna", "Coco", "Nori", "Maple", "Pip", "Momo", "Sunny"}
	var hash uint32
	for _, r := range animalID {
		hash = hash*31 + uint32(r)
	}
	return names[int(hash%uint32(len(names)))]
}

type publicAdventureChoice struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

type publicAdventureStory struct {
	AdventureID    string                  `json:"adventure_id"`
	Theme          string                  `json:"theme"`
	Title          string                  `json:"title"`
	Location       string                  `json:"location"`
	Opening        string                  `json:"opening"`
	EncounterTitle string                  `json:"encounter_title"`
	Encounter      string                  `json:"encounter"`
	CompanionLine  string                  `json:"companion_line"`
	Choices        []publicAdventureChoice `json:"choices"`
	Fiction        bool                    `json:"fiction"`
	Disclaimer     string                  `json:"disclaimer"`
	Source         string                  `json:"source"`
	Degraded       bool                    `json:"degraded,omitempty"`
	ReasonCode     string                  `json:"reason_code,omitempty"`
	PromptVersion  string                  `json:"prompt_version"`
}

func publicAdventure(story services.AdventureResult) publicAdventureStory {
	choices := make([]publicAdventureChoice, 0, len(story.Choices))
	for _, choice := range story.Choices {
		choices = append(choices, publicAdventureChoice{
			ID:          choice.ID,
			Label:       choice.Label,
			Description: choice.Description,
		})
	}
	return publicAdventureStory{
		AdventureID:    story.AdventureID,
		Theme:          story.Theme,
		Title:          story.Title,
		Location:       story.Location,
		Opening:        story.Opening,
		EncounterTitle: story.EncounterTitle,
		Encounter:      story.Encounter,
		CompanionLine:  story.CompanionLine,
		Choices:        choices,
		Fiction:        story.Fiction,
		Disclaimer:     story.Disclaimer,
		Source:         story.Source,
		Degraded:       story.Degraded,
		ReasonCode:     story.ReasonCode,
		PromptVersion:  story.PromptVersion,
	}
}
