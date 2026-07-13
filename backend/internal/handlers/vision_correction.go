package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/services"
	"animalpoke/backend/internal/taxonomy"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const visionCorrectionVersion = "detect-correction-v1"

type visionCorrectionRequest struct {
	DetectInferenceID string `json:"detect_inference_id" binding:"required"`
	TargetID          string `json:"target_id"`
	Species           string `json:"species" binding:"required"`
	SpeciesLabelZH    string `json:"species_label_zh" binding:"required"`
}

type visionCorrectionResponse struct {
	InferenceID       string     `json:"inference_id"`
	ParentInferenceID string     `json:"parent_inference_id"`
	TargetID          string     `json:"target_id"`
	OriginalSpecies   string     `json:"original_species"`
	Species           string     `json:"species"`
	Label             string     `json:"label"`
	Confidence        float64    `json:"confidence"`
	Source            string     `json:"source"`
	ExpiresAt         *time.Time `json:"expires_at,omitempty"`
}

// CorrectDetect creates an immutable, auditable detect credential derived from
// the original model result. Analyze keeps its strict species/credential check
// and accepts the correction only through this new trusted lineage node.
func (h *VisionHandler) CorrectDetect(c *gin.Context) {
	if h.inferenceRepo == nil {
		middleware.WriteError(c, http.StatusServiceUnavailable, "vision_correction_unavailable", "vision correction unavailable", true, nil)
		return
	}

	var req visionCorrectionRequest
	if err := middleware.BindStrictJSON(c, &req); err != nil {
		middleware.WriteBindError(c, err)
		return
	}
	req.DetectInferenceID = strings.TrimSpace(req.DetectInferenceID)
	req.TargetID = strings.TrimSpace(req.TargetID)
	req.Species = strings.TrimSpace(req.Species)
	req.SpeciesLabelZH = strings.TrimSpace(req.SpeciesLabelZH)

	deviceID := middleware.GetDeviceID(c)
	parent, err := h.inferenceRepo.FindForDevice(req.DetectInferenceID, deviceID)
	if err != nil || parent.Kind != "detect" || (parent.Status != "success" && parent.Status != "consumed") {
		middleware.WriteError(c, http.StatusConflict, "detect_inference_invalid", "invalid detect inference", false, nil)
		return
	}
	now := time.Now().UTC()
	if parent.ExpiresAt != nil && !parent.ExpiresAt.IsZero() && now.After(*parent.ExpiresAt) {
		middleware.WriteError(c, http.StatusConflict, "detect_inference_expired", "detect inference expired", false, nil)
		return
	}
	targets, err := parseDetectTargets(parent.ResultJSON)
	if err != nil || len(targets) == 0 {
		middleware.WriteError(c, http.StatusConflict, "detect_targets_missing", "detect has no targets", false, nil)
		return
	}
	if req.TargetID == "" {
		req.TargetID = targets[0].TargetID
	}
	original, err := services.FindTarget(targets, req.TargetID, nil)
	if err != nil {
		middleware.WriteError(c, http.StatusConflict, "target_mismatch", err.Error(), false, nil)
		return
	}

	requestedSpecies, _ := taxonomy.Normalize(req.Species)
	if !taxonomy.Capturable(requestedSpecies) {
		middleware.WriteError(c, http.StatusBadRequest, "species_unsupported", "species not capturable", false, nil)
		return
	}
	correctedSpecies, label, err := services.NormalizeAnimalIdentity(requestedSpecies, req.SpeciesLabelZH)
	if err != nil {
		middleware.WriteError(c, http.StatusBadRequest, "species_label_invalid", err.Error(), false, nil)
		return
	}

	corrected := *original
	corrected.Species = correctedSpecies
	corrected.Label = label
	correction := map[string]interface{}{
		"source":              "user_confirmation",
		"parent_inference_id": parent.InferenceID,
		"target_id":           corrected.TargetID,
		"original_species":    original.Species,
		"original_label":      original.Label,
		"corrected_species":   corrected.Species,
		"corrected_label_zh":  corrected.Label,
		"confirmed_at":        now,
	}
	payload, err := json.Marshal(map[string]interface{}{
		"animals":    []services.DetectBox{corrected},
		"targets":    []services.DetectBox{corrected},
		"correction": correction,
	})
	if err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "vision_correction_encode_failed", "vision correction save failed", true, nil)
		return
	}
	expiresAt := now.Add(2 * time.Hour)
	if parent.ExpiresAt != nil && !parent.ExpiresAt.IsZero() && parent.ExpiresAt.Before(expiresAt) {
		expiresAt = *parent.ExpiresAt
	}
	inferenceID := uuid.NewString()
	if err := h.inferenceRepo.Create(&models.Inference{
		InferenceID:       inferenceID,
		DeviceID:          deviceID,
		Kind:              "detect",
		ParentInferenceID: parent.InferenceID,
		Provider:          "user_confirmation",
		InputDigest:       parent.InputDigest,
		OutputDigest:      sha256Hex(payload),
		ResultJSON:        string(payload),
		Species:           corrected.Species,
		ConfigVersion:     visionCorrectionVersion,
		Status:            "success",
		ExpiresAt:         &expiresAt,
	}); err != nil {
		middleware.WriteError(c, http.StatusInternalServerError, "vision_correction_persist_failed", "vision correction save failed", true, nil)
		return
	}

	c.JSON(http.StatusCreated, visionCorrectionResponse{
		InferenceID:       inferenceID,
		ParentInferenceID: parent.InferenceID,
		TargetID:          corrected.TargetID,
		OriginalSpecies:   original.Species,
		Species:           corrected.Species,
		Label:             corrected.Label,
		Confidence:        corrected.Confidence,
		Source:            "user_confirmation",
		ExpiresAt:         &expiresAt,
	})
}
