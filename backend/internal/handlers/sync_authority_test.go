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
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupSyncAuthority(t *testing.T) (*gin.Engine, *repo.InferenceRepo) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:syncauth_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Animal{}, &models.AuditLog{}, &models.Inference{}))

	animalRepo := repo.NewAnimalRepo(db)
	auditRepo := repo.NewAuditLogRepo(db)
	auditService := services.NewAuditService(animalRepo, auditRepo)
	infRepo := repo.NewInferenceRepo(db)
	handler := NewSyncHandlerFull(animalRepo, auditService, infRepo)

	r := gin.New()
	r.POST("/api/v1/sync/animal", func(c *gin.Context) {
		c.Set("device_id", "dev-1")
		handler.SyncAnimal(c)
	})
	return r, infRepo
}

func TestSyncAnimal_RequiresInference(t *testing.T) {
	r, _ := setupSyncAuthority(t)
	body, _ := json.Marshal(map[string]interface{}{
		"uuid": "u1", "species": "cat", "rarity": 3, "generated_at": time.Now().Format(time.RFC3339),
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/sync/animal", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, 400, w.Code)
}

func TestSyncAnimal_RejectDetectInference(t *testing.T) {
	r, inf := setupSyncAuthority(t)
	require.NoError(t, inf.Create(&models.Inference{
		InferenceID: "det-1", DeviceID: "dev-1", Kind: "detect", Status: "success",
	}))
	body, _ := json.Marshal(map[string]interface{}{
		"uuid": "u2", "species": "cat", "rarity": 5, "generated_at": time.Now().Format(time.RFC3339),
		"inference_request_id": "det-1",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/sync/animal", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, 409, w.Code)
	assert.Contains(t, w.Body.String(), "inference_wrong_kind")
}

func TestSyncAnimal_ServerAuthorityOverwritesClientRarity(t *testing.T) {
	r, inf := setupSyncAuthority(t)
	payload, _ := json.Marshal(map[string]interface{}{
		"species": "cat", "rarity": 2, "hp": 40, "atk": 10, "def": 10, "spd": 10, "class": "Ranger", "element": "Wind",
	})
	exp := time.Now().UTC().Add(time.Hour)
	require.NoError(t, inf.Create(&models.Inference{
		InferenceID: "val-1", DeviceID: "dev-1", Kind: "value", Status: "success",
		Species: "cat", ResultJSON: string(payload), ExpiresAt: &exp,
	}))
	body, _ := json.Marshal(map[string]interface{}{
		"uuid": "u3", "species": "cat", "rarity": 5, "hp": 100, "atk": 50, "def": 50, "spd": 50,
		"class": "Warrior", "element": "Fire",
		"generated_at":         time.Now().Format(time.RFC3339),
		"inference_request_id": "val-1",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/sync/animal", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, 201, w.Code)

	// second consume fails
	body2, _ := json.Marshal(map[string]interface{}{
		"uuid": "u4", "species": "cat", "rarity": 2,
		"generated_at":         time.Now().Format(time.RFC3339),
		"inference_request_id": "val-1",
	})
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/api/v1/sync/animal", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, req2)
	assert.Equal(t, 409, w2.Code)
}
