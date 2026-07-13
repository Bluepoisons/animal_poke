package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"animalpoke/backend/internal/config"
	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/repo"
	"animalpoke/backend/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAdventureHandler(t *testing.T) (*gin.Engine, *gorm.DB, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:adventure_handler_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Animal{},
		&models.AdventureRun{},
		&models.GrowthEvent{},
		&models.ResearcherTrack{},
		&models.CompanionProfile{},
		&models.CompanionMemoryNode{},
	))
	animalID := "550e8400-e29b-41d4-a716-446655440000"
	require.NoError(t, db.Create(&models.Animal{
		UUID: animalID, DeviceID: "dev-adventure", Species: "cat", Breed: "英国短毛猫",
		Rarity: 3, HP: 68, ATK: 27, DEF: 30, SPD: 42, Class: "Ranger", Element: "Wind",
		GeneratedAt: time.Now().UTC(), ServerVersion: 1,
	}).Error)

	animalRepo := repo.NewAnimalRepo(db)
	growthRepo := repo.NewGrowthRepo(db)
	runRepo := repo.NewAdventureRepo(db)
	handler := NewAdventureHandlerWithRepos(
		services.NewAIService(&config.ThirdPartyConfig{}),
		animalRepo,
		growthRepo,
		runRepo,
	)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middleware.ContextDeviceID, "dev-adventure")
		c.Next()
	})
	r.POST("/api/v1/adventures", handler.Generate)
	r.GET("/api/v1/adventures", handler.List)
	r.GET("/api/v1/adventures/:run_id", handler.Get)
	r.POST("/api/v1/adventures/:run_id/choices", handler.CompleteChoice)
	return r, db, animalID
}

func performAdventureRequest(t *testing.T, r http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		payload, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(payload)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestAdventureHandlerGenerateCompleteAndIdempotency(t *testing.T) {
	r, db, animalID := setupAdventureHandler(t)
	body := map[string]any{
		"animal_uuid":  animalID,
		"theme":        "mistwood",
		"operation_id": "operation-adventure-1",
	}
	w := performAdventureRequest(t, r, http.MethodPost, "/api/v1/adventures", body)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var generated map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &generated))
	runID := generated["adventure_id"].(string)
	assert.NotEmpty(t, runID)
	assert.NotContains(t, generated, "souvenir")
	choices := generated["choices"].([]any)
	require.Len(t, choices, 3)
	assert.NotContains(t, choices[0].(map[string]any), "outcome")
	assert.NotContains(t, choices[0].(map[string]any), "bond_delta")

	replayed := performAdventureRequest(t, r, http.MethodPost, "/api/v1/adventures", body)
	require.Equal(t, http.StatusOK, replayed.Code, replayed.Body.String())
	var replayedBody map[string]any
	require.NoError(t, json.Unmarshal(replayed.Body.Bytes(), &replayedBody))
	assert.Equal(t, runID, replayedBody["adventure_id"])
	var runCount int64
	require.NoError(t, db.Model(&models.AdventureRun{}).Count(&runCount).Error)
	assert.EqualValues(t, 1, runCount)

	completed := performAdventureRequest(t, r, http.MethodPost, "/api/v1/adventures/"+runID+"/choices", map[string]any{"choice_id": "curiosity"})
	require.Equal(t, http.StatusOK, completed.Code, completed.Body.String())
	var completion map[string]any
	require.NoError(t, json.Unmarshal(completed.Body.Bytes(), &completion))
	assert.Equal(t, "completed", completion["status"])
	assert.NotEmpty(t, completion["outcome"])
	assert.NotEmpty(t, completion["souvenir"])
	companion := completion["companion"].(map[string]any)
	assert.EqualValues(t, 6, companion["bond_xp"])

	retry := performAdventureRequest(t, r, http.MethodPost, "/api/v1/adventures/"+runID+"/choices", map[string]any{"choice_id": "curiosity"})
	require.Equal(t, http.StatusOK, retry.Code, retry.Body.String())
	var profile models.CompanionProfile
	require.NoError(t, db.Where("animal_uuid = ?", animalID).First(&profile).Error)
	assert.EqualValues(t, 6, profile.BondXP)

	conflict := performAdventureRequest(t, r, http.MethodPost, "/api/v1/adventures/"+runID+"/choices", map[string]any{"choice_id": "kindness"})
	assert.Equal(t, http.StatusConflict, conflict.Code)

	detail := performAdventureRequest(t, r, http.MethodGet, "/api/v1/adventures/"+runID, nil)
	require.Equal(t, http.StatusOK, detail.Code, detail.Body.String())
	var detailBody map[string]any
	require.NoError(t, json.Unmarshal(detail.Body.Bytes(), &detailBody))
	story := detailBody["story"].(map[string]any)
	assert.NotContains(t, story, "souvenir")
	assert.Equal(t, "completed", detailBody["status"])
	assert.NotEmpty(t, detailBody["outcome"])

	list := performAdventureRequest(t, r, http.MethodGet, "/api/v1/adventures?animal_uuid="+animalID, nil)
	require.Equal(t, http.StatusOK, list.Code, list.Body.String())
	var listBody map[string]any
	require.NoError(t, json.Unmarshal(list.Body.Bytes(), &listBody))
	assert.Len(t, listBody["items"].([]any), 1)
}

func TestAdventureHandlerChoiceRollsBackWhenGrowthFails(t *testing.T) {
	r, db, animalID := setupAdventureHandler(t)
	generated := performAdventureRequest(t, r, http.MethodPost, "/api/v1/adventures", map[string]any{
		"animal_uuid":  animalID,
		"theme":        "mistwood",
		"operation_id": "operation-adventure-rollback",
	})
	require.Equal(t, http.StatusCreated, generated.Code, generated.Body.String())
	var story map[string]any
	require.NoError(t, json.Unmarshal(generated.Body.Bytes(), &story))
	runID := story["adventure_id"].(string)

	require.NoError(t, db.Model(&models.Animal{}).
		Where("uuid = ?", animalID).
		Update("deleted_at", time.Now().UTC()).Error)

	completed := performAdventureRequest(t, r, http.MethodPost, "/api/v1/adventures/"+runID+"/choices", map[string]any{"choice_id": "curiosity"})
	require.Equal(t, http.StatusNotFound, completed.Code, completed.Body.String())

	var run models.AdventureRun
	require.NoError(t, db.Where("run_id = ?", runID).First(&run).Error)
	assert.Equal(t, "generated", run.Status)
	assert.Empty(t, run.SelectedChoiceID)
	var eventCount int64
	require.NoError(t, db.Model(&models.GrowthEvent{}).Where("source_id = ?", runID).Count(&eventCount).Error)
	assert.Zero(t, eventCount)
}

func TestAdventureHandlerRejectsMismatchedIdempotencyKey(t *testing.T) {
	r, db, animalID := setupAdventureHandler(t)
	payload, err := json.Marshal(map[string]any{
		"animal_uuid": animalID, "theme": "mistwood", "operation_id": "operation-in-body",
	})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/adventures", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(middleware.HeaderIdempotencyKey, "operation-in-header")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "invalid_adventure_operation")

	var count int64
	require.NoError(t, db.Model(&models.AdventureRun{}).Count(&count).Error)
	assert.Zero(t, count)
}
