// Package handlers MB3: AI 推理处理函数(LLM 数值生成)。
package handlers

import (
	"log/slog"
	"net/http"

	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/services"

	"github.com/gin-gonic/gin"
)

// ValueHandler LLM 数值生成处理器。
type ValueHandler struct {
	llmService *services.LLMService
}

// NewValueHandler 构造 ValueHandler。
func NewValueHandler(llmService *services.LLMService) *ValueHandler {
	return &ValueHandler{llmService: llmService}
}

type valueRequest struct {
	Species            string `json:"species" binding:"required"`
	Breed              string `json:"breed"`
	Color              string `json:"color"`
	BodyType           string `json:"body_type"`
	SubjectCompleteness int   `json:"subject_completeness"`
	Clarity            int    `json:"clarity"`
	Lighting           int    `json:"lighting"`
	Composition        int    `json:"composition"`
	Pose               int    `json:"pose"`
	Angle              int    `json:"angle"`
}

// Generate POST /value/generate 调 LLM 生成稀有度/六维属性/叙事。
func (h *ValueHandler) Generate(c *gin.Context) {
	var req valueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: species is required"})
		return
	}

	deviceID := middleware.GetDeviceID(c)
	slog.Info("LLM 数值生成请求", "device_id", deviceID, "species", req.Species)

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

	result, err := h.llmService.GenerateValue(input)
	if err != nil {
		slog.Error("LLM 数值生成失败", "device_id", deviceID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "value generation failed"})
		return
	}

	c.JSON(http.StatusOK, result)
}
