// Package handlers MB3: AI 推理处理函数(VLM 检测/分析)。
package handlers

import (
	"io"
	"log/slog"
	"net/http"

	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/services"

	"github.com/gin-gonic/gin"
)

// VisionHandler VLM 推理处理器。
type VisionHandler struct {
	aiService *services.AIService
}

// NewVisionHandler 构造 VisionHandler。
func NewVisionHandler(aiService *services.AIService) *VisionHandler {
	return &VisionHandler{aiService: aiService}
}

// Detect POST /vision/detect 上传帧, VLM 检测动物。
// 原始帧内存态转发, 推理后即时销毁不落盘。
func (h *VisionHandler) Detect(c *gin.Context) {
	file, header, err := c.Request.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image file is required"})
		return
	}
	defer file.Close()

	imageData, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read image"})
		return
	}

	deviceID := middleware.GetDeviceID(c)
	slog.Info("AI 检测请求", "device_id", deviceID, "filename", header.Filename, "size", len(imageData))

	result, err := h.aiService.Detect(imageData, header.Filename)
	if err != nil {
		slog.Error("AI 检测失败", "device_id", deviceID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "detection failed"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// Analyze POST /vision/analyze 上传帧, VLM 深度分析。
func (h *VisionHandler) Analyze(c *gin.Context) {
	file, header, err := c.Request.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image file is required"})
		return
	}
	defer file.Close()

	imageData, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read image"})
		return
	}

	deviceID := middleware.GetDeviceID(c)
	slog.Info("AI 分析请求", "device_id", deviceID, "filename", header.Filename, "size", len(imageData))

	result, err := h.aiService.Analyze(imageData, header.Filename)
	if err != nil {
		slog.Error("AI 分析失败", "device_id", deviceID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "analysis failed"})
		return
	}

	c.JSON(http.StatusOK, result)
}
