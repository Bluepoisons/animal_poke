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

	"animalpoke/backend/internal/config"
	"animalpoke/backend/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, _ := w.CreateFormFile(fieldName, filename)
	io.Copy(part, bytes.NewReader(data))
	w.Close()
	return w.FormDataContentType(), &buf
}

func createMultipartBodyWithFields(filename string, data []byte, fields map[string]string) (string, *bytes.Buffer) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, _ := w.CreateFormFile("image", filename)
	_, _ = io.Copy(part, bytes.NewReader(data))
	for k, v := range fields {
		_ = w.WriteField(k, v)
	}
	_ = w.Close()
	return w.FormDataContentType(), &buf
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

func solidPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: uint8(x * 7), G: uint8(y * 5), B: 40, A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

// jpegWithFakeEXIF builds a valid JPEG and injects an APP1 Exif segment after SOI.
func jpegWithFakeEXIF(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := range 4 {
		for x := range 4 {
			img.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}))
	raw := buf.Bytes()
	require.True(t, bytes.HasPrefix(raw, []byte{0xFF, 0xD8}))

	// APP1: marker + length (2 bytes, includes length field) + "Exif\0\0" + pad
	// length = 2 + 6 + 8 = 16 → 0x0010
	app1 := []byte{
		0xFF, 0xE1, 0x00, 0x10,
		'E', 'x', 'i', 'f', 0x00, 0x00,
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
	}
	out := make([]byte, 0, len(raw)+len(app1))
	out = append(out, raw[:2]...) // SOI
	out = append(out, app1...)
	out = append(out, raw[2:]...)
	return out
}

// local json helper
func jsonUnmarshal(b []byte, v any) error {
	return json.Unmarshal(b, v)
}
