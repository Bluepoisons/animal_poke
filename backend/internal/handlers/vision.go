// Package handlers MB3: AI 推理处理函数(VLM 检测/分析)。
package handlers

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/repo"
	"animalpoke/backend/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "golang.org/x/image/webp"
)

// VisionHandler VLM 推理处理器。
type VisionHandler struct {
	aiService      *services.AIService
	inferenceRepo  *repo.InferenceRepo
	deviceRepo     *repo.DeviceRepo
	maxBytes       int64
	maxPixels      int
	requireConsent bool
	consentVer     string
}

// VisionHandlerOptions 可选依赖。
type VisionHandlerOptions struct {
	InferenceRepo  *repo.InferenceRepo
	DeviceRepo     *repo.DeviceRepo
	MaxBytes       int64
	MaxPixels      int
	RequireConsent bool
	ConsentVersion string
}

// NewVisionHandler 构造 VisionHandler。
func NewVisionHandler(aiService *services.AIService) *VisionHandler {
	return &VisionHandler{
		aiService: aiService,
		maxBytes:  5 << 20,
		maxPixels: 12_000_000,
	}
}

// NewVisionHandlerWithOptions 完整构造。
func NewVisionHandlerWithOptions(aiService *services.AIService, opts VisionHandlerOptions) *VisionHandler {
	h := NewVisionHandler(aiService)
	h.inferenceRepo = opts.InferenceRepo
	h.deviceRepo = opts.DeviceRepo
	if opts.MaxBytes > 0 {
		h.maxBytes = opts.MaxBytes
	}
	if opts.MaxPixels > 0 {
		h.maxPixels = opts.MaxPixels
	}
	h.requireConsent = opts.RequireConsent
	h.consentVer = opts.ConsentVersion
	if h.consentVer == "" {
		h.consentVer = "v1"
	}
	return h
}

// Detect POST /vision/detect
func (h *VisionHandler) Detect(c *gin.Context) {
	h.handleVision(c, "detect")
}

// Analyze POST /vision/analyze
func (h *VisionHandler) Analyze(c *gin.Context) {
	h.handleVision(c, "analyze")
}

func (h *VisionHandler) handleVision(c *gin.Context, kind string) {
	deviceID := middleware.GetDeviceID(c)

	if h.requireConsent && h.deviceRepo != nil {
		ok, err := h.deviceRepo.HasValidConsent(deviceID, h.consentVer)
		if err != nil || !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "consent required", "reason_code": "consent_missing"})
			return
		}
	}

	// 限制请求体
	if c.Request.Body != nil {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.maxBytes+1024)
	}

	file, header, err := c.Request.FormFile("image")
	if err != nil {
		if strings.Contains(err.Error(), "http: request body too large") {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "image too large", "code": 413})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "image file is required"})
		return
	}
	defer file.Close()

	limited := io.LimitReader(file, h.maxBytes+1)
	imageData, err := io.ReadAll(limited)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read image"})
		return
	}
	if int64(len(imageData)) > h.maxBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "image too large", "code": 413})
		return
	}

	if err := validateImage(imageData, h.maxPixels); err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "unsupported") {
			status = http.StatusUnsupportedMediaType
		}
		c.JSON(status, gin.H{"error": err.Error(), "code": status})
		return
	}

	slog.Info("AI 视觉请求", "kind", kind, "device_id", deviceID, "filename", header.Filename, "size", len(imageData))
	start := time.Now()

	var (
		result interface{}
		model  string
		pver   string
		digest = sha256Hex(imageData)
	)

	switch kind {
	case "detect":
		r, err := h.aiService.DetectContext(c.Request.Context(), imageData, header.Filename)
		if err != nil {
			slog.Error("AI 检测失败", "device_id", deviceID, "err", err)
			WriteProviderError(c, err, "detection failed")
			return
		}
		model, pver = r.Model, r.PromptVersion
		// 不保留原图；仅摘要
		if h.inferenceRepo != nil {
			id := uuid.NewString()
			exp := time.Now().UTC().Add(2 * time.Hour)
			_ = h.inferenceRepo.Create(&models.Inference{
				InferenceID:   id,
				DeviceID:      deviceID,
				Kind:          "detect",
				Provider:      "vision",
				Model:         model,
				PromptVersion: pver,
				InputDigest:   digest,
				OutputDigest:  sha256Hex([]byte(fmt.Sprintf("%d", len(r.Animals)))),
				ConfigVersion: "detect-v1",
				Status:        "success",
				DurationMs:    time.Since(start).Milliseconds(),
				ExpiresAt:     &exp,
			})
			r.InferenceID = id
		}
		result = r
	default:
		r, err := h.aiService.AnalyzeContext(c.Request.Context(), imageData, header.Filename)
		if err != nil {
			slog.Error("AI 分析失败", "device_id", deviceID, "err", err)
			WriteProviderError(c, err, "analysis failed")
			return
		}
		model, pver = r.Model, r.PromptVersion
		if h.inferenceRepo != nil {
			id := uuid.NewString()
			exp := time.Now().UTC().Add(2 * time.Hour)
			parent := c.PostForm("parent_inference_id")
			if parent == "" {
				parent = c.PostForm("detect_inference_id")
			}
			_ = h.inferenceRepo.Create(&models.Inference{
				InferenceID:       id,
				DeviceID:          deviceID,
				Kind:              "analyze",
				ParentInferenceID: parent,
				Provider:          "vision",
				Model:             model,
				PromptVersion:     pver,
				InputDigest:       digest,
				OutputDigest:      sha256Hex([]byte(r.Breed + r.Color)),
				ConfigVersion:     "analyze-v1",
				Status:            "success",
				DurationMs:        time.Since(start).Milliseconds(),
				ExpiresAt:         &exp,
			})
			r.InferenceID = id
		}
		result = r
	}

	// imageData 出作用域后由 GC 回收，不落盘
	_ = imageData
	c.JSON(http.StatusOK, result)
}

func validateImage(data []byte, maxPixels int) error {
	if len(data) < 12 {
		return fmt.Errorf("unsupported media type")
	}
	// magic bytes
	ct := http.DetectContentType(data)
	allowed := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/webp": true,
	}
	// webp magic
	if bytes.HasPrefix(data, []byte("RIFF")) && len(data) >= 12 && bytes.Equal(data[8:12], []byte("WEBP")) {
		ct = "image/webp"
	}
	if !allowed[ct] {
		return fmt.Errorf("unsupported media type: %s", ct)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		// webp 解码可能失败于无完整解码器；magic 通过则允许
		if ct == "image/webp" {
			return nil
		}
		return fmt.Errorf("invalid image: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return fmt.Errorf("invalid image dimensions")
	}
	if maxPixels > 0 && cfg.Width*cfg.Height > maxPixels {
		return fmt.Errorf("image pixels exceed limit")
	}
	return nil
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
