// Package handlers MB3: AI 推理处理函数(VLM 检测/分析)。
package handlers

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/repo"
	"animalpoke/backend/internal/services"
	"animalpoke/backend/internal/taxonomy"

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
			c.JSON(http.StatusInternalServerError, gin.H{"error": "detection failed", "reason_code": "detect_failed"})
			return
		}
		model, pver = r.Model, r.PromptVersion
		// 不保留原图；仅摘要 + 目标列表
		if h.inferenceRepo != nil {
			id := uuid.NewString()
			exp := time.Now().UTC().Add(2 * time.Hour)
			payload, _ := json.Marshal(map[string]interface{}{
				"targets": r.Targets,
				"animals": r.Animals,
			})
			topSpecies := ""
			if len(r.Targets) > 0 {
				topSpecies = r.Targets[0].Species
			}
			_ = h.inferenceRepo.Create(&models.Inference{
				InferenceID:   id,
				DeviceID:      deviceID,
				Kind:          "detect",
				Provider:      "vision",
				Model:         model,
				PromptVersion: pver,
				InputDigest:   digest,
				OutputDigest:  sha256Hex(payload),
				ResultJSON:    string(payload),
				Species:       topSpecies,
				ConfigVersion: "detect-v2",
				Status:        "success",
				DurationMs:    time.Since(start).Milliseconds(),
				ExpiresAt:     &exp,
			})
			r.InferenceID = id
		}
		result = r
	default:
		// AP-020: 多目标一致性 — 需引用 detect + target_id/box
		detectInfID := c.PostForm("detect_inference_id")
		if detectInfID == "" {
			detectInfID = c.PostForm("parent_inference_id")
		}
		targetID := c.PostForm("target_id")
		claimedSpecies := strings.TrimSpace(c.PostForm("species"))
		box, boxOK, boxErr := parseOptionalBox(c)
		if boxErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": boxErr.Error(), "reason_code": "invalid_box"})
			return
		}

		var locked *services.DetectBox
		if h.inferenceRepo != nil {
			if detectInfID == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "detect_inference_id required", "reason_code": "detect_inference_required"})
				return
			}
			if targetID == "" && !boxOK {
				c.JSON(http.StatusBadRequest, gin.H{"error": "target_id or box required", "reason_code": "target_required"})
				return
			}
			parent, err := h.inferenceRepo.FindForDevice(detectInfID, deviceID)
			if err != nil || parent.Kind != "detect" || (parent.Status != "success" && parent.Status != "consumed") {
				c.JSON(http.StatusConflict, gin.H{"error": "invalid detect inference", "reason_code": "detect_inference_invalid"})
				return
			}
			if parent.ExpiresAt != nil && !parent.ExpiresAt.IsZero() && time.Now().UTC().After(*parent.ExpiresAt) {
				c.JSON(http.StatusConflict, gin.H{"error": "detect inference expired", "reason_code": "detect_inference_expired"})
				return
			}
			targets, err := parseDetectTargets(parent.ResultJSON)
			if err != nil || len(targets) == 0 {
				c.JSON(http.StatusConflict, gin.H{"error": "detect has no targets", "reason_code": "detect_targets_missing"})
				return
			}
			var boxPtr *services.BoundingBox
			if boxOK {
				boxPtr = &box
			}
			locked, err = services.FindTarget(targets, targetID, boxPtr)
			if err != nil {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "reason_code": "target_mismatch"})
				return
			}
			if claimedSpecies != "" {
				norm, _ := taxonomy.Normalize(claimedSpecies)
				if norm != locked.Species {
					c.JSON(http.StatusConflict, gin.H{
						"error":       "species does not match selected target",
						"reason_code": "target_mismatch",
						"expected":    locked.Species,
						"got":         norm,
					})
					return
				}
			}
		} else if claimedSpecies != "" {
			// 无 inference 仓储时仅规范化声明物种（测试/降级）
			norm, _ := taxonomy.Normalize(claimedSpecies)
			if !taxonomy.Capturable(norm) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "species not capturable", "reason_code": "species_unsupported"})
				return
			}
			locked = &services.DetectBox{Species: norm, TargetID: targetID}
			if boxOK {
				locked.BoundingBox = box
			}
		}

		r, err := h.aiService.AnalyzeContext(c.Request.Context(), imageData, header.Filename)
		if err != nil {
			slog.Error("AI 分析失败", "device_id", deviceID, "err", err)
			// 校验失败与模型失败区分
			msg := err.Error()
			if strings.Contains(msg, "out of range") || strings.Contains(msg, "missing") ||
				strings.Contains(msg, "json") || strings.Contains(msg, "markdown") || strings.Contains(msg, "multiple") {
				c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid analysis output", "reason_code": "analysis_invalid", "detail": msg})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "analysis failed", "reason_code": "analyze_failed"})
			return
		}
		if locked != nil {
			r.Species = locked.Species
			r.TargetID = locked.TargetID
			r.DetectInferenceID = detectInfID
			bb := locked.BoundingBox
			r.Box = &bb
		}
		model, pver = r.Model, r.PromptVersion
		if h.inferenceRepo != nil {
			id := uuid.NewString()
			exp := time.Now().UTC().Add(2 * time.Hour)
			payload, _ := json.Marshal(map[string]interface{}{
				"species":              r.Species,
				"target_id":            r.TargetID,
				"detect_inference_id":  detectInfID,
				"breed":                r.Breed,
				"color":                r.Color,
				"body_type":            r.BodyType,
				"quality_score":        r.QualityScore,
				"subject_completeness": r.SubjectCompleteness,
				"clarity":              r.Clarity,
				"lighting":             r.Lighting,
				"composition":          r.Composition,
				"pose":                 r.Pose,
				"angle":                r.Angle,
				"box":                  r.Box,
			})
			_ = h.inferenceRepo.Create(&models.Inference{
				InferenceID:       id,
				DeviceID:          deviceID,
				Kind:              "analyze",
				ParentInferenceID: detectInfID,
				Provider:          "vision",
				Model:             model,
				PromptVersion:     pver,
				InputDigest:       digest,
				OutputDigest:      sha256Hex(payload),
				ResultJSON:        string(payload),
				Species:           r.Species,
				ConfigVersion:     "analyze-v2",
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

func parseOptionalBox(c *gin.Context) (services.BoundingBox, bool, error) {
	// box as JSON: box={"x":..,"y":..,"width":..,"height":..}
	if raw := strings.TrimSpace(c.PostForm("box")); raw != "" {
		var bb services.BoundingBox
		if err := json.Unmarshal([]byte(raw), &bb); err != nil {
			return services.BoundingBox{}, false, fmt.Errorf("invalid box json")
		}
		if err := services.ValidateBoundingBox(bb); err != nil {
			return services.BoundingBox{}, false, err
		}
		return bb, true, nil
	}
	// discrete fields box_x / box_y / box_width / box_height
	xs, ys, ws, hs := c.PostForm("box_x"), c.PostForm("box_y"), c.PostForm("box_width"), c.PostForm("box_height")
	if xs == "" && ys == "" && ws == "" && hs == "" {
		// also accept bounding_box_* aliases
		xs, ys = c.PostForm("bounding_box_x"), c.PostForm("bounding_box_y")
		ws, hs = c.PostForm("bounding_box_width"), c.PostForm("bounding_box_height")
	}
	if xs == "" && ys == "" && ws == "" && hs == "" {
		return services.BoundingBox{}, false, nil
	}
	parse := func(s, name string) (float64, error) {
		if s == "" {
			return 0, fmt.Errorf("missing %s", name)
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid %s", name)
		}
		return v, nil
	}
	x, err := parse(xs, "box_x")
	if err != nil {
		return services.BoundingBox{}, false, err
	}
	y, err := parse(ys, "box_y")
	if err != nil {
		return services.BoundingBox{}, false, err
	}
	w, err := parse(ws, "box_width")
	if err != nil {
		return services.BoundingBox{}, false, err
	}
	h, err := parse(hs, "box_height")
	if err != nil {
		return services.BoundingBox{}, false, err
	}
	bb := services.BoundingBox{X: x, Y: y, Width: w, Height: h}
	if err := services.ValidateBoundingBox(bb); err != nil {
		return services.BoundingBox{}, false, err
	}
	return bb, true, nil
}

func parseDetectTargets(resultJSON string) ([]services.DetectBox, error) {
	if strings.TrimSpace(resultJSON) == "" {
		return nil, fmt.Errorf("empty result json")
	}
	var env struct {
		Targets []services.DetectBox `json:"targets"`
		Animals []services.DetectBox `json:"animals"`
	}
	if err := json.Unmarshal([]byte(resultJSON), &env); err != nil {
		return nil, err
	}
	if len(env.Targets) > 0 {
		return env.Targets, nil
	}
	return env.Animals, nil
}

