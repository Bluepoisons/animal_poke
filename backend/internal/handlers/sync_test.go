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
	"animalpoke/backend/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupSyncTest(t *testing.T) (*gin.Engine, *SyncHandler) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)
	err = db.AutoMigrate(&models.Animal{}, &models.AuditLog{})
	assert.NoError(t, err)

	animalRepo := repo.NewAnimalRepo(db)
	auditRepo := repo.NewAuditLogRepo(db)
	auditService := services.NewAuditService(animalRepo, auditRepo)
	handler := NewSyncHandler(animalRepo, auditService)

	r := gin.New()
	r.POST("/api/v1/sync/animal", handler.SyncAnimal)
	return r, handler
}

func TestSyncAnimal_MissingFields(t *testing.T) {
	r, _ := setupSyncTest(t)

	body, _ := json.Marshal(map[string]interface{}{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/sync/animal", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, 400, w.Code)
}

func TestSyncAnimal_Success(t *testing.T) {
	r, _ := setupSyncTest(t)

	body, _ := json.Marshal(map[string]interface{}{
		"uuid":         "550e8400-e29b-41d4-a716-446655440001",
		"species":      "cat",
		"breed":        "British Shorthair",
		"rarity":       3,
		"hp":           65,
		"atk":          32,
		"def":          28,
		"spd":          40,
		"class":        "Ranger",
		"element":      "Wind",
		"latitude":     39.9042,
		"longitude":    116.4074,
		"generated_at": time.Now().Format(time.RFC3339),
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/sync/animal", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, 201, w.Code)

	var resp syncResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "synced", resp.Status)
	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440001", resp.UUID)
}

func TestSyncAnimal_Duplicate(t *testing.T) {
	r, _ := setupSyncTest(t)

	body, _ := json.Marshal(map[string]interface{}{
		"uuid":         "550e8400-e29b-41d4-a716-446655440002",
		"species":      "dog",
		"rarity":       2,
		"generated_at": time.Now().Format(time.RFC3339),
	})

	// 第一次应成功
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/sync/animal", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, 201, w.Code)

	// 第二次应返回冲突
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/api/v1/sync/animal", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, 409, w.Code)
}

func TestSyncAnimal_InvalidDateFormat(t *testing.T) {
	r, _ := setupSyncTest(t)

	body, _ := json.Marshal(map[string]interface{}{
		"uuid":         "bad-date",
		"species":      "cat",
		"rarity":       1,
		"generated_at": "not-a-date",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/sync/animal", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, 400, w.Code)
}
