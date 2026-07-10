package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/repo"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAuthHandlerTest(t *testing.T) (*gin.Engine, *AuthHandler) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// 迁移完整 Device 模型
	err = db.AutoMigrate(&models.Device{})
	assert.NoError(t, err)

	deviceRepo := repo.NewDeviceRepo(db)
	handler := NewAuthHandler(deviceRepo, "test-secret", 24*time.Hour)

	r := gin.New()
	r.POST("/api/v1/auth/device", handler.DeviceAuth)
	return r, handler
}

func TestDeviceAuth_Success(t *testing.T) {
	r, _ := setupAuthHandlerTest(t)

	body, _ := json.Marshal(map[string]string{"device_id": "test-device-001"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/device", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var resp authResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NotEmpty(t, resp.Token)
	assert.NotEmpty(t, resp.ExpiresAt)
}

func TestDeviceAuth_MissingDeviceID(t *testing.T) {
	r, _ := setupAuthHandlerTest(t)

	body, _ := json.Marshal(map[string]string{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/device", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
}

func TestDeviceAuth_EmptyDeviceID(t *testing.T) {
	r, _ := setupAuthHandlerTest(t)

	body, _ := json.Marshal(map[string]string{"device_id": ""})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/device", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)
}

func TestDeviceAuth_SameDeviceReturnsToken(t *testing.T) {
	r, _ := setupAuthHandlerTest(t)

	body, _ := json.Marshal(map[string]string{"device_id": "same-device"})
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/auth/device", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)

		var resp authResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NotEmpty(t, resp.Token)
	}
}
