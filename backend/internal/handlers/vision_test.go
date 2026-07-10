package handlers

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"animalpoke/backend/internal/config"
	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/repo"
	"animalpoke/backend/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupVisionTest() (*gin.Engine, *VisionHandler) {
	gin.SetMode(gin.TestMode)
	cfg := &config.ThirdPartyConfig{} // 空 Key, 使用 mock
	svc := services.NewVisionService(cfg)
	handler := NewVisionHandler(svc)
	r := gin.New()
	r.POST("/api/v1/vision/detect", handler.Detect)
	r.POST("/api/v1/vision/analyze", handler.Analyze)
	return r, handler
}

func createMultipartBody(fieldName, filename string, data []byte) (string, *bytes.Buffer) {
	return createMultipartBodyFields(fieldName, filename, data, nil)
}

func createMultipartBodyFields(fieldName, filename string, data []byte, fields map[string]string) (string, *bytes.Buffer) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, _ := w.CreateFormFile(fieldName, filename)
	io.Copy(part, bytes.NewReader(data))
	for k, v := range fields {
		_ = w.WriteField(k, v)
	}
	w.Close()
	return w.FormDataContentType(), &buf
}

func setupVisionWithRepo(t *testing.T) (*gin.Engine, *VisionHandler, *repo.InferenceRepo) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:vision_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Inference{}))
	inf := repo.NewInferenceRepo(db)
	cfg := &config.ThirdPartyConfig{}
	svc := services.NewVisionService(cfg)
	handler := NewVisionHandlerWithOptions(svc, VisionHandlerOptions{InferenceRepo: inf})
	r := gin.New()
	r.POST("/api/v1/vision/detect", handler.Detect)
	r.POST("/api/v1/vision/analyze", handler.Analyze)
	return r, handler, inf
}

func seedDetectInference(t *testing.T, inf *repo.InferenceRepo, id, device string, targets []map[string]interface{}) {
	t.Helper()
	payload, _ := json.Marshal(map[string]interface{}{"targets": targets, "animals": targets})
	exp := time.Now().UTC().Add(time.Hour)
	require.NoError(t, inf.Create(&models.Inference{
		InferenceID: id, DeviceID: device, Kind: "detect", Status: "success",
		ResultJSON: string(payload), Species: targets[0]["species"].(string), ExpiresAt: &exp,
	}))
}


func TestVisionDetect_MissingFile(t *testing.T) {
	r, _ := setupVisionTest()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/vision/detect", nil)
	req.Header.Set("Content-Type", "multipart/form-data")
	r.ServeHTTP(w, req)
	assert.Equal(t, 400, w.Code)
}

func TestVisionDetect_RejectsNonImage(t *testing.T) {
	r, _ := setupVisionTest()
	ct, buf := createMultipartBody("image", "test.bin", []byte("fake-image-data"))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/vision/detect", buf)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)
	assert.True(t, w.Code == 400 || w.Code == 415)
}

func TestVisionDetect_Success(t *testing.T) {
	r, _ := setupVisionTest()

	png := tinyPNG()
	ct, buf := createMultipartBody("image", "test.png", png)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/vision/detect", buf)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var result services.DetectResult
	require.NoError(t, jsonUnmarshal(w.Body.Bytes(), &result))
	assert.NotEmpty(t, result.Animals)
	assert.Equal(t, "cat", result.Animals[0].Species)
}

func TestVisionAnalyze_Success(t *testing.T) {
	r, _ := setupVisionTest()

	png := tinyPNG()
	ct, buf := createMultipartBody("image", "cat.png", png)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/vision/analyze", buf)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var result services.AnalysisResult
	require.NoError(t, jsonUnmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, "British Shorthair", result.Breed)
}

// TestMinimizeForProvider_StripsEXIF ensures re-encode drops APP1 Exif markers.
func TestMinimizeForProvider_StripsEXIF(t *testing.T) {
	withExif := jpegWithFakeEXIF(t)
	require.True(t, bytes.Contains(withExif, []byte("Exif")), "fixture must contain Exif marker")

	out, w, h, err := minimizeForProvider(withExif, 12_000_000, nil)
	require.NoError(t, err)
	assert.Greater(t, w, 0)
	assert.Greater(t, h, 0)
	assert.True(t, bytes.HasPrefix(out, []byte{0xFF, 0xD8, 0xFF}), "output must be JPEG")
	assert.False(t, bytes.Contains(out, []byte("Exif")), "re-encoded JPEG must not contain Exif")
	assert.False(t, bytes.Contains(out, []byte{0xFF, 0xE1}), "re-encoded JPEG must not contain APP1")
}

// TestValidateImage_RejectsBadWebP: RIFF/WEBP magic alone is not enough.
func TestValidateImage_RejectsBadWebP(t *testing.T) {
	// Truncated / malicious RIFF WEBP that cannot fully decode.
	bad := []byte{
		'R', 'I', 'F', 'F',
		0x08, 0x00, 0x00, 0x00,
		'W', 'E', 'B', 'P',
		'V', 'P', '8', ' ',
		0x00, 0x00, 0x00, 0x00,
	}
	err := validateImage(bad, 12_000_000)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid image")
}

// TestValidateImage_RejectsHugePixels enforces maxPixels after dimension discovery.
func TestValidateImage_RejectsHugePixels(t *testing.T) {
	// 8x8 image → 64 pixels; limit 16 must reject.
	img := solidPNG(t, 8, 8)
	err := validateImage(img, 16)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pixels exceed")
}

// TestMinimizeForProvider_CropPath crops before re-encode.
func TestMinimizeForProvider_CropPath(t *testing.T) {
	src := solidPNG(t, 40, 20)
	crop := &cropBox{X: 0.25, Y: 0.0, W: 0.5, H: 1.0}
	out, w, h, err := minimizeForProvider(src, 12_000_000, crop)
	require.NoError(t, err)
	assert.True(t, bytes.HasPrefix(out, []byte{0xFF, 0xD8, 0xFF}))
	// 40*0.5 = 20 width, full height 20
	assert.Equal(t, 20, w)
	assert.Equal(t, 20, h)

	// Round-trip decode to confirm JPEG is valid.
	_, err = jpeg.Decode(bytes.NewReader(out))
	require.NoError(t, err)
}

func TestVisionAnalyze_WithCrop(t *testing.T) {
	r, _ := setupVisionTest()
	png := solidPNG(t, 32, 32)
	ct, buf := createMultipartBodyWithFields("cat.png", png, map[string]string{
		"crop_x": "0.1",
		"crop_y": "0.1",
		"crop_w": "0.5",
		"crop_h": "0.5",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/vision/analyze", buf)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var result services.AnalysisResult
	require.NoError(t, jsonUnmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, "British Shorthair", result.Breed)
}

func TestVisionAnalyze_InvalidCrop(t *testing.T) {
	r, _ := setupVisionTest()
	png := solidPNG(t, 16, 16)
	ct, buf := createMultipartBodyWithFields("cat.png", png, map[string]string{
		"crop_x": "0.8",
		"crop_y": "0.8",
		"crop_w": "0.5",
		"crop_h": "0.5",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/vision/analyze", buf)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)
	assert.Equal(t, 400, w.Code)
}

func TestDetectAllowedImageType_Strict(t *testing.T) {
	_, err := detectAllowedImageType([]byte("not-an-image"))
	require.Error(t, err)

	_, err = detectAllowedImageType(tinyPNG())
	require.NoError(t, err)
}

// 1x1 PNG
func tinyPNG() []byte {
	return []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xde, 0x00, 0x00, 0x00,
		0x0c, 0x49, 0x44, 0x41, 0x54, 0x08, 0xd7, 0x63, 0xf8, 0xff, 0xff, 0x3f,
		0x00, 0x05, 0xfe, 0x02, 0xfe, 0xdc, 0xcc, 0x59, 0xe7, 0x00, 0x00, 0x00,
		0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
	}
}


func TestVisionDetect_ReturnsTargetsAndStoresJSON(t *testing.T) {
	r, _, inf := setupVisionWithRepo(t)
	// device id via middleware is empty in bare router; Create still works with ""
	png := tinyPNG()
	ct, buf := createMultipartBody("image", "test.png", png)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/vision/detect", buf)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var result services.DetectResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.NotEmpty(t, result.Animals)
	assert.NotEmpty(t, result.Targets)
	assert.Equal(t, result.Animals[0].TargetID, result.Targets[0].TargetID)
	assert.NotEmpty(t, result.InferenceID)

	stored, err := inf.Find(result.InferenceID)
	require.NoError(t, err)
	assert.Contains(t, stored.ResultJSON, "targets")
	assert.Contains(t, stored.ResultJSON, "target_id")
}

func TestVisionAnalyze_RequiresDetectWhenRepoPresent(t *testing.T) {
	r, _, _ := setupVisionWithRepo(t)
	png := tinyPNG()
	ct, buf := createMultipartBody("image", "cat.png", png)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/vision/analyze", buf)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "detect_inference_required")
}

func TestVisionAnalyze_TargetMismatch(t *testing.T) {
	r, _, inf := setupVisionWithRepo(t)
	seedDetectInference(t, inf, "det-mm", "", []map[string]interface{}{
		{"species": "cat", "target_id": "0", "confidence": 0.9, "bounding_box": map[string]float64{"x": 0.1, "y": 0.1, "width": 0.3, "height": 0.4}},
		{"species": "dog", "target_id": "1", "confidence": 0.85, "bounding_box": map[string]float64{"x": 0.5, "y": 0.2, "width": 0.3, "height": 0.4}},
	})
	png := tinyPNG()
	// claim dog while selecting cat target
	ct, buf := createMultipartBodyFields("image", "cat.png", png, map[string]string{
		"detect_inference_id": "det-mm",
		"target_id":           "0",
		"species":             "dog",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/vision/analyze", buf)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)
	assert.Equal(t, 409, w.Code)
	assert.Contains(t, w.Body.String(), "target_mismatch")
}

func TestVisionAnalyze_MultiTargetSelection(t *testing.T) {
	r, _, inf := setupVisionWithRepo(t)
	seedDetectInference(t, inf, "det-multi", "", []map[string]interface{}{
		{"species": "cat", "target_id": "0", "confidence": 0.9, "bounding_box": map[string]float64{"x": 0.1, "y": 0.1, "width": 0.3, "height": 0.4}},
		{"species": "dog", "target_id": "1", "confidence": 0.85, "bounding_box": map[string]float64{"x": 0.5, "y": 0.2, "width": 0.3, "height": 0.4}},
	})
	png := tinyPNG()
	ct, buf := createMultipartBodyFields("image", "cat.png", png, map[string]string{
		"detect_inference_id": "det-multi",
		"target_id":           "0",
		"species":             "cat",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/vision/analyze", buf)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	var result services.AnalysisResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, "cat", result.Species)
	assert.Equal(t, "0", result.TargetID)
	assert.Equal(t, "det-multi", result.DetectInferenceID)
	assert.NotEmpty(t, result.InferenceID)
}

func TestVisionAnalyze_InvalidBox(t *testing.T) {
	r, _, inf := setupVisionWithRepo(t)
	seedDetectInference(t, inf, "det-box", "", []map[string]interface{}{
		{"species": "cat", "target_id": "0", "confidence": 0.9, "bounding_box": map[string]float64{"x": 0.1, "y": 0.1, "width": 0.3, "height": 0.4}},
	})
	png := tinyPNG()
	ct, buf := createMultipartBodyFields("image", "cat.png", png, map[string]string{
		"detect_inference_id": "det-box",
		"box_x":               "0.1",
		"box_y":               "0.1",
		"box_width":           "0.95",
		"box_height":          "0.95", // x+w > 1
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/vision/analyze", buf)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "invalid_box")
}

func TestVisionAnalyze_SelectByBox(t *testing.T) {
	r, _, inf := setupVisionWithRepo(t)
	seedDetectInference(t, inf, "det-iou", "", []map[string]interface{}{
		{"species": "cat", "target_id": "0", "confidence": 0.9, "bounding_box": map[string]float64{"x": 0.1, "y": 0.1, "width": 0.3, "height": 0.4}},
		{"species": "dog", "target_id": "1", "confidence": 0.85, "bounding_box": map[string]float64{"x": 0.5, "y": 0.2, "width": 0.3, "height": 0.4}},
	})
	png := tinyPNG()
	ct, buf := createMultipartBodyFields("image", "dog.png", png, map[string]string{
		"detect_inference_id": "det-iou",
		"species":             "dog",
		"box":                 `{"x":0.52,"y":0.22,"width":0.28,"height":0.38}`,
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/vision/analyze", buf)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
	var result services.AnalysisResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, "dog", result.Species)
	assert.Equal(t, "1", result.TargetID)
}
