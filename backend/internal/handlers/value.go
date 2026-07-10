// Package handlers MB3: AI 推理处理函数(LLM 数值生成)。
package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/repo"
	"animalpoke/backend/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ValueHandler LLM 数值生成处理器。
type ValueHandler struct {
	aiService     *services.AIService
	inferenceRepo *repo.InferenceRepo
}

// NewValueHandler 构造 ValueHandler。
func NewValueHandler(aiService *services.AIService) *ValueHandler {
	return &ValueHandler{aiService: aiService}
}

// NewValueHandlerWithRepo 带 provenance。
func NewValueHandlerWithRepo(aiService *services.AIService, inf *repo.InferenceRepo) *ValueHandler {
	return &ValueHandler{aiService: aiService, inferenceRepo: inf}
}

type valueRequest struct {
	Species             string `json:"species" binding:"required"`
	Breed               string `json:"breed"`
	Color               string `json:"color"`
	BodyType            string `json:"body_type"`
	SubjectCompleteness int    `json:"subject_completeness"`
	Clarity             int    `json:"clarity"`
	Lighting            int    `json:"lighting"`
	Composition         int    `json:"composition"`
	Pose                int    `json:"pose"`
	Angle               int    `json:"angle"`
}

// Generate POST /value/generate
func (h *ValueHandler) Generate(c *gin.Context) {
	var req valueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: species is required"})
		return
	}

	input := services.ValueInput{
		Species:             req.Species,
		Breed:               req.Breed,
		Color:               req.Color,
		BodyType:            req.BodyType,
		SubjectCompleteness: req.SubjectCompleteness,
		Clarity:             req.Clarity,
		Lighting:            req.Lighting,
		Composition:         req.Composition,
		Pose:                req.Pose,
		Angle:               req.Angle,
	}
	if err := input.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	deviceID := middleware.GetDeviceID(c)
	slog.Info("AI 数值生成请求", "device_id", deviceID, "species", req.Species)
	start := time.Now()

	result, err := h.aiService.GenerateValueContext(c.Request.Context(), input)
	if err != nil {
		slog.Error("AI 数值生成失败", "device_id", deviceID, "err", err)
		if err.Error() == "invalid input: "+err.Error() || // fallback
			true {
			// 区分 400/500
			if result == nil && (len(err.Error()) > 0) {
				// invalid input already checked
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "value generation failed"})
		return
	}

	if h.inferenceRepo != nil {
		id := uuid.NewString()
		_ = h.inferenceRepo.Create(&models.Inference{
			InferenceID:   id,
			DeviceID:      deviceID,
			Kind:          "value",
			Provider:      "llm",
			Model:         result.Model,
			PromptVersion: result.PromptVersion,
			InputDigest:   sha256Hex([]byte(fmt.Sprintf("%s|%s|%s", req.Species, req.Breed, req.Color))),
			OutputDigest:  sha256Hex([]byte(fmt.Sprintf("%d|%d|%s", result.Rarity, result.HP, result.Class))),
			Status:        "success",
			DurationMs:    time.Since(start).Milliseconds(),
		})
		result.InferenceID = id
	}

	c.JSON(http.StatusOK, result)
}
