package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/repo"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupNarrative(t *testing.T) (*gin.Engine, *repo.NarrativeRepo) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:narr_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Device{},
		&models.NarrativeNode{}, &models.NarrativeChoice{}, &models.NarrativeProgress{},
		&models.NarrativeSeenState{}, &models.NarrativeChoiceLog{},
		&models.StoryFragment{}, &models.StoryFragmentUnlock{}, &models.ClueState{},
	))
	deviceRepo := repo.NewDeviceRepo(db)
	authH := NewAuthHandler(deviceRepo, "test-secret", 24*time.Hour)
	nr := repo.NewNarrativeRepo(db)
	require.NoError(t, nr.SeedContent())
	nh := NewNarrativeHandler(nr)

	r := gin.New()
	r.POST("/api/v1/auth/device", authH.DeviceAuth)
	auth := r.Group("/api/v1")
	auth.Use(middleware.JWTAuthWithChecker("test-secret", "animal-poke", "animal-poke-client", deviceCheckerAdapter{deviceRepo}))
	{
		auth.GET("/narrative/nodes/:node_id", nh.GetNode)
		auth.GET("/narrative/progress", nh.GetProgress)
		auth.POST("/narrative/choices", nh.SubmitChoice)
		auth.POST("/narrative/fail-forward", nh.FailForward)
		auth.POST("/narrative/observation", nh.ObservationEvent)
		auth.POST("/narrative/clues", nh.UpdateClue)
		auth.GET("/narrative/clues", nh.ListClues)
	}
	return r, nr
}

func narrAuth(t *testing.T, r *gin.Engine, deviceID string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"device_id": deviceID})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/device", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code, w.Body.String())
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp["token"].(string)
}

func narrJSON(t *testing.T, r *gin.Engine, method, path, token string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	var body []byte
	if payload != nil {
		body, _ = json.Marshal(payload)
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	r.ServeHTTP(w, req)
	return w
}

func TestNarrative_ProgressAndChoiceIdempotent(t *testing.T) {
	r, _ := setupNarrative(t)
	tok := narrAuth(t, r, "narr-dev-1")

	w := narrJSON(t, r, "GET", "/api/v1/narrative/progress?chapter=ch1", tok, nil)
	require.Equal(t, 200, w.Code, w.Body.String())
	var prog map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &prog))
	p := prog["progress"].(map[string]any)
	assert.Equal(t, "ch1_intro", p["current_node_id"])

	// move to choice node
	w = narrJSON(t, r, "POST", "/api/v1/narrative/choices", tok, map[string]string{
		"chapter_id": "ch1", "choice_id": "ch1_intro_next", "operation_id": "op-intro-1",
	})
	require.Equal(t, 201, w.Code, w.Body.String())

	// choose slow
	w = narrJSON(t, r, "POST", "/api/v1/narrative/choices", tok, map[string]string{
		"chapter_id": "ch1", "choice_id": "ch1_c_slow", "operation_id": "op-slow-1",
	})
	require.Equal(t, 201, w.Code, w.Body.String())

	// duplicate operation
	w = narrJSON(t, r, "POST", "/api/v1/narrative/choices", tok, map[string]string{
		"chapter_id": "ch1", "choice_id": "ch1_c_slow", "operation_id": "op-slow-1",
	})
	require.Equal(t, 200, w.Code, w.Body.String())
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["idempotent"])

	// illegal skip
	w = narrJSON(t, r, "POST", "/api/v1/narrative/choices", tok, map[string]string{
		"chapter_id": "ch1", "choice_id": "ch1_c_quick", "operation_id": "op-bad",
	})
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestNarrative_FailForwardAndFragments(t *testing.T) {
	r, _ := setupNarrative(t)
	tok := narrAuth(t, r, "narr-dev-2")

	w := narrJSON(t, r, "POST", "/api/v1/narrative/fail-forward", tok, map[string]any{
		"miss_count": 3, "reason": "miss",
	})
	require.Equal(t, 200, w.Code, w.Body.String())
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	node := resp["node"].(map[string]any)
	assert.Equal(t, "ff_miss_3", node["node_id"])
	assert.Equal(t, "authored_canon", node["layer"])

	w = narrJSON(t, r, "POST", "/api/v1/narrative/observation", tok, map[string]any{
		"operation_id": "obs-1", "species": "cat", "is_first_species": true, "observation_count": 1,
	})
	require.Equal(t, 200, w.Code, w.Body.String())
	var fr map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &fr))
	unlocked := fr["unlocked"].([]any)
	assert.NotEmpty(t, unlocked)

	// replay same op — no double unlock for same fragments
	w = narrJSON(t, r, "POST", "/api/v1/narrative/observation", tok, map[string]any{
		"operation_id": "obs-1", "species": "cat", "is_first_species": true, "observation_count": 1,
	})
	require.Equal(t, 200, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &fr))
	assert.Empty(t, fr["unlocked"])
}

func TestNarrative_ClueStates(t *testing.T) {
	r, _ := setupNarrative(t)
	tok := narrAuth(t, r, "narr-dev-3")
	w := narrJSON(t, r, "POST", "/api/v1/narrative/clues", tok, map[string]string{
		"clue_id": "rain_shadow", "status": "pending", "source": "player",
	})
	require.Equal(t, 200, w.Code, w.Body.String())
	w = narrJSON(t, r, "GET", "/api/v1/narrative/clues", tok, nil)
	require.Equal(t, 200, w.Code)
}
