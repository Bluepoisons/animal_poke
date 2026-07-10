// Package handlers MB3: AI 推理处理函数(VLM 检测/分析)。
package handlers

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
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

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "golang.org/x/image/webp"
)

// Image privacy / retention (AP-019):
//
//   - Upload bytes exist only in request-scoped memory ([]byte); never written to disk,
//     object storage, or durable caches.
//   - Before any provider call we fully decode and re-encode as baseline JPEG (quality 85).
//     That re-encode strips EXIF / ICC / XMP / orientation chunks and unknown ancillary data.
//   - Analyze may optionally crop to a normalized box so only the selected animal region
//     is forwarded (reduces background faces / plates / homes in the provider payload).
//   - Logs and Inference rows store sha256(digest of minimized JPEG) + width/height only —
//     never original filename, crop coordinates, GPS, or pixel payloads.
//   - After the handler returns, image slices leave scope for GC. Provider-side retention
//     is governed by the third-party DPA; we only send the minimized JPEG bytes.
//
// Supported inbound formats: image/jpeg, image/png, image/webp (strict magic + full decode).

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

// cropBox is a normalized [0,1] rectangle (x,y,w,h). Coordinates are never logged.
type cropBox struct {
	X, Y, W, H float64
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

	file, _, err := c.Request.FormFile("image")
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

	var crop *cropBox
	if kind == "analyze" {
		crop, err = parseOptionalCrop(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": 400})
			return
		}
	}

	// Strict decode + re-encode to JPEG (strips EXIF/metadata); optional crop for analyze.
	minimized, width, height, err := minimizeForProvider(imageData, h.maxPixels, crop)
	if err != nil {
		status := http.StatusBadRequest
		msg := err.Error()
		if strings.Contains(msg, "unsupported") {
			status = http.StatusUnsupportedMediaType
		}
		c.JSON(status, gin.H{"error": msg, "code": status})
		return
	}
	// Drop raw upload ASAP; only minimized JPEG proceeds to provider.
	imageData = nil

	digest := sha256Hex(minimized)
	// Privacy: digest + dimensions only — never filename, crop coords, or raw size of original.
	slog.Info("AI 视觉请求",
		"kind", kind,
		"device_id", deviceID,
		"digest", digest,
		"width", width,
		"height", height,
	)
	start := time.Now()

	// Provider always receives re-encoded JPEG under a fixed name (no client filename).
	const providerName = "image.jpg"

	var (
		result interface{}
		model  string
		pver   string
	)

	switch kind {
	case "detect":
		r, err := h.aiService.DetectContext(c.Request.Context(), minimized, providerName)
		if err != nil {
			slog.Error("AI 检测失败", "device_id", deviceID, "digest", digest, "err", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "detection failed"})
			return
		}
		model, pver = r.Model, r.PromptVersion
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
		r, err := h.aiService.AnalyzeContext(c.Request.Context(), minimized, providerName)
		if err != nil {
			slog.Error("AI 分析失败", "device_id", deviceID, "digest", digest, "err", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "analysis failed"})
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

	// minimized 出作用域后由 GC 回收，不落盘
	_ = minimized
	c.JSON(http.StatusOK, result)
}

// parseOptionalCrop reads optional normalized crop box form fields for analyze.
// Fields: crop_x, crop_y, crop_w, crop_h (fractions of image size, 0..1).
// Missing all fields → nil (full frame). Partial/invalid → error.
func parseOptionalCrop(c *gin.Context) (*cropBox, error) {
	rawX := strings.TrimSpace(c.PostForm("crop_x"))
	rawY := strings.TrimSpace(c.PostForm("crop_y"))
	rawW := strings.TrimSpace(c.PostForm("crop_w"))
	rawH := strings.TrimSpace(c.PostForm("crop_h"))
	if rawX == "" && rawY == "" && rawW == "" && rawH == "" {
		return nil, nil
	}
	if rawX == "" || rawY == "" || rawW == "" || rawH == "" {
		return nil, fmt.Errorf("crop box requires crop_x, crop_y, crop_w, crop_h")
	}
	x, err1 := strconv.ParseFloat(rawX, 64)
	y, err2 := strconv.ParseFloat(rawY, 64)
	w, err3 := strconv.ParseFloat(rawW, 64)
	h, err4 := strconv.ParseFloat(rawH, 64)
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		return nil, fmt.Errorf("invalid crop box numbers")
	}
	const eps = 1e-6
	if x < -eps || y < -eps || w <= eps || h <= eps {
		return nil, fmt.Errorf("invalid crop box range")
	}
	if x > 1+eps || y > 1+eps || w > 1+eps || h > 1+eps {
		return nil, fmt.Errorf("invalid crop box range")
	}
	if x+w > 1+eps || y+h > 1+eps {
		return nil, fmt.Errorf("crop box exceeds image bounds")
	}
	// Clamp into [0,1] after soft epsilon checks.
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	return &cropBox{X: x, Y: y, W: w, H: h}, nil
}

// detectAllowedImageType returns a canonical type for supported formats only.
// Does not trust Content-Type headers or incomplete WebP magic alone.
func detectAllowedImageType(data []byte) (string, error) {
	if len(data) < 12 {
		return "", fmt.Errorf("unsupported media type")
	}
	switch {
	case bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}):
		return "image/jpeg", nil
	case bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}):
		return "image/png", nil
	case bytes.HasPrefix(data, []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")):
		return "image/webp", nil
	default:
		ct := http.DetectContentType(data)
		return "", fmt.Errorf("unsupported media type: %s", ct)
	}
}

// minimizeForProvider strictly decodes inbound jpeg/png/webp, enforces maxPixels,
// optionally crops, and re-encodes to JPEG (strips EXIF and other metadata).
// Decode failures (truncated / malicious / unreadable WebP) are always rejected —
// matching magic alone is never sufficient.
func minimizeForProvider(data []byte, maxPixels int, crop *cropBox) (out []byte, width, height int, err error) {
	if _, err := detectAllowedImageType(data); err != nil {
		return nil, 0, 0, err
	}

	// Cheap dimension check before full raster allocation when possible.
	if cfg, _, cfgErr := image.DecodeConfig(bytes.NewReader(data)); cfgErr == nil {
		if cfg.Width <= 0 || cfg.Height <= 0 {
			return nil, 0, 0, fmt.Errorf("invalid image dimensions")
		}
		if maxPixels > 0 {
			// Guard against overflow: reject if either dim is huge before multiply.
			if cfg.Width > maxPixels || cfg.Height > maxPixels || cfg.Width*cfg.Height > maxPixels {
				return nil, 0, 0, fmt.Errorf("image pixels exceed limit")
			}
		}
	}

	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		// Strict: WebP magic match does NOT bypass decode failures.
		return nil, 0, 0, fmt.Errorf("invalid image: %w", err)
	}
	switch format {
	case "jpeg", "png", "webp":
		// ok
	default:
		return nil, 0, 0, fmt.Errorf("unsupported media type: %s", format)
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= 0 || h <= 0 {
		return nil, 0, 0, fmt.Errorf("invalid image dimensions")
	}
	if maxPixels > 0 && (w > maxPixels || h > maxPixels || w*h > maxPixels) {
		return nil, 0, 0, fmt.Errorf("image pixels exceed limit")
	}

	if crop != nil {
		img, w, h, err = applyNormalizedCrop(img, *crop)
		if err != nil {
			return nil, 0, 0, err
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return nil, 0, 0, fmt.Errorf("re-encode failed: %w", err)
	}
	return buf.Bytes(), w, h, nil
}

// applyNormalizedCrop crops img to the normalized box and returns a fresh NRGBA.
func applyNormalizedCrop(img image.Image, crop cropBox) (image.Image, int, int, error) {
	b := img.Bounds()
	fw, fh := float64(b.Dx()), float64(b.Dy())
	x0 := b.Min.X + int(crop.X*fw)
	y0 := b.Min.Y + int(crop.Y*fh)
	x1 := b.Min.X + int((crop.X+crop.W)*fw)
	y1 := b.Min.Y + int((crop.Y+crop.H)*fh)
	if x1 <= x0 {
		x1 = x0 + 1
	}
	if y1 <= y0 {
		y1 = y0 + 1
	}
	if x0 < b.Min.X {
		x0 = b.Min.X
	}
	if y0 < b.Min.Y {
		y0 = b.Min.Y
	}
	if x1 > b.Max.X {
		x1 = b.Max.X
	}
	if y1 > b.Max.Y {
		y1 = b.Max.Y
	}
	if x1-x0 < 1 || y1-y0 < 1 {
		return nil, 0, 0, fmt.Errorf("crop region empty")
	}
	rect := image.Rect(x0, y0, x1, y1)
	dst := image.NewNRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
	draw.Draw(dst, dst.Bounds(), img, rect.Min, draw.Src)
	return dst, dst.Bounds().Dx(), dst.Bounds().Dy(), nil
}

// validateImage is retained for direct unit tests of the strict gate (no re-encode).
// Production path uses minimizeForProvider.
func validateImage(data []byte, maxPixels int) error {
	_, _, _, err := minimizeForProvider(data, maxPixels, nil)
	return err
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
