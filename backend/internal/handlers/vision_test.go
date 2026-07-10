package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"animalpoke/backend/internal/config"
	"animalpoke/backend/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
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

// minimalJPEG 最小合法 JPEG（SOI+EOI），magic 通过。
var minimalJPEG = []byte{0xFF, 0xD8, 0xFF, 0xD9}

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

	// 构造可通过 DecodeConfig 的小 PNG
	png := tinyPNG()
	ct, buf := createMultipartBody("image", "test.png", png)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/vision/detect", buf)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var result services.DetectResult
	json.Unmarshal(w.Body.Bytes(), &result)
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
	json.Unmarshal(w.Body.Bytes(), &result)
	assert.Equal(t, "British Shorthair", result.Breed)
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
