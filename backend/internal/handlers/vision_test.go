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

func TestVisionDetect_MissingFile(t *testing.T) {
	r, _ := setupVisionTest()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/vision/detect", nil)
	req.Header.Set("Content-Type", "multipart/form-data")
	r.ServeHTTP(w, req)
	assert.Equal(t, 400, w.Code)
}

func TestVisionDetect_Success(t *testing.T) {
	r, _ := setupVisionTest()

	ct, buf := createMultipartBody("image", "test.jpg", []byte("fake-image-data"))
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

	ct, buf := createMultipartBody("image", "cat.jpg", []byte("fake-image-data"))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/vision/analyze", buf)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	var result services.AnalysisResult
	json.Unmarshal(w.Body.Bytes(), &result)
	assert.Equal(t, "British Shorthair", result.Breed)
}
