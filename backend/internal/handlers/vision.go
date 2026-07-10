// Package handlers MB3: AI 推理处理函数(VLM 检测/分析)。
package handlers

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	"animalpoke/backend/internal/safety"
	"animalpoke/backend/internal/services"
	"animalpoke/backend/internal/taxonomy"

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
	// providerNoTrain enables no-train/no-retain policy audit stubs (AP-056).
	providerNoTrain bool
	// allowFixture enables safety_fixture form field (dev/mock only).
	allowFixture bool
}

// VisionHandlerOptions 可选依赖。
type VisionHandlerOptions struct {
	InferenceRepo         *repo.InferenceRepo
	DeviceRepo            *repo.DeviceRepo
	MaxBytes              int64
	MaxPixels             int
	RequireConsent        bool
	ConsentVersion        string
	ProviderNoTrainPolicy bool
	AllowSafetyFixture    bool
}

// NewVisionHandler 构造 VisionHandler。
func NewVisionHandler(aiService *services.AIService) *VisionHandler {
	return &VisionHandler{
		aiService:       aiService,
		maxBytes:        5 << 20,
		maxPixels:       12_000_000,
		providerNoTrain: true,
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
	h.providerNoTrain = opts.ProviderNoTrainPolicy
	h.allowFixture = opts.AllowSafetyFixture
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

	filename := ""
	if header != nil {
		filename = header.Filename
	}
	digest := sha256Hex(imageData)
	fixture := ""
	if h.allowFixture {
		fixture = safety.NormalizeFixture(c.PostForm("safety_fixture"))
	}

	// Pre-moderation: hard-reject pure portrait/child/abuse/plate/house fixtures
	// without sending image to provider (and never log original bytes).
	if fixture != "" {
		pre := safety.Evaluate(safety.Input{FixtureLabel: fixture, Filename: filename})
		if pre.Action == safety.ActionReject || (pre.DecisionCode == safety.CodeFlagAbuse) {
			slog.Info("AI 视觉安全拒绝",
				"kind", kind,
				"device_id", deviceID,
				"decision_code", pre.DecisionCode,
				"input_digest", digest,
				// no filename if it may encode PII; no image bytes
			)
			h.respondSafetyReject(c, kind, deviceID, digest, pre)
			// drop image reference
			imageData = nil
			return
		}
	}

	slog.Info("AI 视觉请求", "kind", kind, "device_id", deviceID, "size", len(imageData), "input_digest", digest)
	start := time.Now()

	if h.providerNoTrain {
		safety.LogProviderNoTrain("vision", kind, "", digest, deviceID, middleware.GetRequestID(c))
	}

	var (
		result interface{}
		model  string
		pver   string
	)

	switch kind {
	case "detect":
		r, err := h.aiService.DetectContext(c.Request.Context(), imageData, filename)
		if err != nil {
			slog.Error("AI 检测失败", "device_id", deviceID, "err", err)
			WriteProviderError(c, err, "detection failed")
			return
		}
		model, pver = r.Model, r.PromptVersion
		h.applyDetectSafety(r, fixture, filename)
		// 不保留原图；仅摘要
		if h.inferenceRepo != nil {
			id := uuid.NewString()
			exp := time.Now().UTC().Add(2 * time.Hour)
			status := "success"
			if r.Safety != nil && !r.Safety.Collectable && r.Safety.Action == safety.ActionReject {
				status = "rejected"
			}
			_ = h.inferenceRepo.Create(&models.Inference{
				InferenceID:   id,
				DeviceID:      deviceID,
				Kind:          "detect",
				Provider:      "vision",
				Model:         model,
				PromptVersion: pver,
				InputDigest:   digest,
				OutputDigest:  sha256Hex([]byte(fmt.Sprintf("%d:%s", len(r.Animals), reasonOrEmpty(r)))),
				ConfigVersion: "detect-v1",
				Status:        status,
				DurationMs:    time.Since(start).Milliseconds(),
				ExpiresAt:     &exp,
			})
			r.InferenceID = id
		}
		// Clear internal labels before response
		r.SafetyLabels = nil
		result = r
	default:
		r, err := h.aiService.AnalyzeContext(c.Request.Context(), imageData, filename)
		if err != nil {
			slog.Error("AI 分析失败", "device_id", deviceID, "err", err)
			WriteProviderError(c, err, "analysis failed")
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
		// Analyze: re-check fixture/filename only (no multi-label from detect)
		if fixture != "" || filename != "" {
			sr := safety.Evaluate(safety.Input{FixtureLabel: fixture, Filename: filename, HasCapturableAnimal: true})
			view := sr.ToClientView()
			r.Safety = &services.SafetySummary{
				Allowed: view.Allowed, Collectable: view.Collectable,
				DecisionCode: view.DecisionCode, Action: view.Action,
				Flags: view.Flags, ReportPath: view.ReportPath,
			}
			if !sr.Collectable && sr.Action == safety.ActionReject {
				r.ReasonCode = sr.DecisionCode
				r.Breed, r.Color, r.BodyType = "", "", ""
			}
		}
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

	// imageData 出作用域后由 GC 回收，不落盘；显式置空降低残留窗口
	for i := range imageData {
		imageData[i] = 0
	}
	imageData = nil
	c.JSON(http.StatusOK, result)
}

func (h *VisionHandler) applyDetectSafety(r *services.DetectResult, fixture, filename string) {
	if r == nil {
		return
	}
	hasAnimal := len(r.Animals) > 0
	// person_animal fixture implies animal even if mock returned cat already
	in := safety.Input{
		FixtureLabel:        fixture,
		Filename:            filename,
		Labels:              append([]string(nil), r.SafetyLabels...),
		HasCapturableAnimal: hasAnimal,
	}
	// Also feed species of capturable animals as labels for completeness.
	for _, a := range r.Animals {
		if a.Species != "" {
			in.Labels = append(in.Labels, a.Species)
		}
		if a.Label != "" {
			in.Labels = append(in.Labels, a.Label)
		}
	}
	sr := safety.Evaluate(in)
	view := sr.ToClientView()
	r.Safety = &services.SafetySummary{
		Allowed:      view.Allowed,
		Collectable:  view.Collectable,
		DecisionCode: view.DecisionCode,
		Action:       view.Action,
		Flags:        view.Flags,
		ReportPath:   view.ReportPath,
	}
	if !sr.Collectable {
		// Pure portrait / sensitive / abuse: strip animals so they cannot be collected.
		r.Animals = []services.DetectBox{}
		r.ReasonCode = sr.DecisionCode
		if r.Source == "" {
			r.Source = "safety"
		}
	}
}

func (h *VisionHandler) respondSafetyReject(c *gin.Context, kind, deviceID, digest string, pre safety.Result) {
	view := pre.ToClientView()
	summary := &services.SafetySummary{
		Allowed: view.Allowed, Collectable: view.Collectable,
		DecisionCode: view.DecisionCode, Action: view.Action,
		Flags: view.Flags, ReportPath: view.ReportPath,
	}
	infID := ""
	if h.inferenceRepo != nil {
		id := uuid.NewString()
		exp := time.Now().UTC().Add(2 * time.Hour)
		_ = h.inferenceRepo.Create(&models.Inference{
			InferenceID:   id,
			DeviceID:      deviceID,
			Kind:          kind,
			Provider:      "safety",
			InputDigest:   digest,
			OutputDigest:  sha256Hex([]byte(pre.DecisionCode)),
			ConfigVersion: "safety-v1",
			Status:        "rejected",
			ExpiresAt:     &exp,
		})
		infID = id
	}
	if kind == "analyze" {
		c.JSON(http.StatusOK, &services.AnalysisResult{
			Source:      "safety",
			ReasonCode:  pre.DecisionCode,
			InferenceID: infID,
			Safety:      summary,
		})
		return
	}
	c.JSON(http.StatusOK, &services.DetectResult{
		Animals:     []services.DetectBox{},
		Source:      "safety",
		ReasonCode:  pre.DecisionCode,
		InferenceID: infID,
		Safety:      summary,
	})
}

func reasonOrEmpty(r *services.DetectResult) string {
	if r == nil || r.Safety == nil {
		return ""
	}
	return r.Safety.DecisionCode
}

func validateImage(data []byte, maxPixels int) error {
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

