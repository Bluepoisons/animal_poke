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
	visionService *services.VisionService
}

// NewVisionHandler 构造 VisionHandler。
func NewVisionHandler(visionService *services.VisionService) *VisionHandler {
	return &VisionHandler{visionService: visionService}
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
	slog.Info("VLM 检测请求", "device_id", deviceID, "filename", header.Filename, "size", len(imageData))

	result, err := h.visionService.Detect(imageData, header.Filename)
	if err != nil {
		slog.Error("VLM 检测失败", "device_id", deviceID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "detection failed"})
		return
	}

	// imageData 超出作用域后 GC 回收, 不落盘
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
	slog.Info("VLM 分析请求", "device_id", deviceID, "filename", header.Filename, "size", len(imageData))

	result, err := h.visionService.Analyze(imageData, header.Filename)
	if err != nil {
		slog.Error("VLM 分析失败", "device_id", deviceID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "analysis failed"})
		return
	}

	c.JSON(http.StatusOK, result)
}
