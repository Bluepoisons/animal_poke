package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/repo"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupGrowthHandler(t *testing.T) (*gin.Engine, *repo.GrowthRepo, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dsn := fmt.Sprintf("file:%s/gh.db?_busy_timeout=5000&_journal_mode=WAL&cache=shared", t.TempDir())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Animal{},
		&models.ResearcherTrack{},
		&models.GrowthEvent{},
		&models.CompanionProfile{},
		&models.CompanionMemoryNode{},
		&models.GrowthResetAudit{},
	))
	grepo := repo.NewGrowthRepo(db)
	h := NewGrowthHandler(grepo)
	r := gin.New()
	auth := r.Group("/api/v1")
	auth.Use(func(c *gin.Context) {
		dev := c.GetHeader("X-Test-Device")
		if dev == "" {
			dev = "dev-1"
		}
		c.Set(middleware.ContextKeyDeviceID, dev)
		if acc := c.GetHeader("X-Test-Account"); acc != "" {
			c.Set(middleware.ContextKeyAccountID, acc)
		}
		c.Next()
	})
	{
		auth.GET("/growth/catalog", h.GetCatalog)
		auth.GET("/growth/researcher", h.GetResearcher)
		auth.POST("/growth/events", h.PostEvent)
		auth.GET("/growth/events", h.ListEvents)
		auth.GET("/growth/companions", h.ListCompanions)
		auth.GET("/growth/companions/:animal_uuid", h.GetCompanion)
		auth.POST("/growth/reset", h.Reset)
	}
	return r, grepo, db
}

func postGrowth(r *gin.Engine, path string, body map[string]interface{}, device, account string) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if device != "" {
		req.Header.Set("X-Test-Device", device)
	}
	if account != "" {
		req.Header.Set("X-Test-Account", account)
	}
	r.ServeHTTP(w, req)
	return w
}

func getGrowth(r *gin.Engine, path, device, account string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, path, nil)
	if device != "" {
		req.Header.Set("X-Test-Device", device)
	}
	if account != "" {
		req.Header.Set("X-Test-Account", account)
	}
	r.ServeHTTP(w, req)
	return w
}

func TestGrowthHandler_Catalog(t *testing.T) {
	r, _, _ := setupGrowthHandler(t)
	w := getGrowth(r, "/api/v1/growth/catalog", "dev-1", "")
	assert.Equal(t, 200, w.Code, w.Body.String())
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, models.GrowthConfigVersion, resp["config_version"])
	rules := resp["rules"].(map[string]interface{})
	assert.Equal(t, true, rules["no_paid_power"])
	assert.Equal(t, true, rules["no_decay"])
	assert.Equal(t, float64(3), rules["min_visible_nodes_per_companion"])
}

func TestGrowthHandler_EventIdempotentAndTracks(t *testing.T) {
	r, _, _ := setupGrowthHandler(t)
	body := map[string]interface{}{
		"event_id": "h-e1", "kind": "photo_quality", "source_type": "capture",
	}
	w1 := postGrowth(r, "/api/v1/growth/events", body, "dev-1", "")
	assert.Equal(t, http.StatusCreated, w1.Code, w1.Body.String())
	var resp1 map[string]interface{}
	require.NoError(t, json.Unmarshal(w1.Body.Bytes(), &resp1))
	assert.Equal(t, false, resp1["idempotent"])
	assert.Equal(t, true, resp1["combat_unchanged"])

	w2 := postGrowth(r, "/api/v1/growth/events", body, "dev-1", "")
	assert.Equal(t, http.StatusOK, w2.Code)
	var resp2 map[string]interface{}
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp2))
	assert.Equal(t, true, resp2["idempotent"])

	wr := getGrowth(r, "/api/v1/growth/researcher", "dev-1", "")
	assert.Equal(t, 200, wr.Code)
	var research map[string]interface{}
	require.NoError(t, json.Unmarshal(wr.Body.Bytes(), &research))
	tracks := research["tracks"].([]interface{})
	assert.Len(t, tracks, 3)
}

func TestGrowthHandler_RejectPaidPower(t *testing.T) {
	r, _, _ := setupGrowthHandler(t)
	w := postGrowth(r, "/api/v1/growth/events", map[string]interface{}{
		"event_id": "pay1", "kind": "paid_power",
	}, "dev-1", "")
	assert.Equal(t, http.StatusForbidden, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "growth_paid_power_forbidden", resp["reason_code"])
}

func TestGrowthHandler_CompanionNodes(t *testing.T) {
	r, _, db := setupGrowthHandler(t)
	animalID := uuid.NewString()
	require.NoError(t, db.Create(&models.Animal{
		UUID: animalID, DeviceID: "dev-1", Species: "dog", Rarity: 1,
		HP: 50, ATK: 10, DEF: 10, SPD: 10, GeneratedAt: time.Now().UTC(), ServerVersion: 1,
	}).Error)

	w := postGrowth(r, "/api/v1/growth/events", map[string]interface{}{
		"event_id": "c-int-1", "kind": "companion_interact", "animal_uuid": animalID,
	}, "dev-1", "")
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	wg := getGrowth(r, "/api/v1/growth/companions/"+animalID, "dev-1", "")
	assert.Equal(t, 200, wg.Code, wg.Body.String())
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(wg.Body.Bytes(), &resp))
	assert.GreaterOrEqual(t, int(resp["visible_nodes"].(float64)), 3)
	nodes := resp["nodes"].([]interface{})
	assert.GreaterOrEqual(t, len(nodes), 3)
	assert.Equal(t, true, resp["combat_unchanged"])
}

func TestGrowthHandler_CrossDeviceSync(t *testing.T) {
	r, _, _ := setupGrowthHandler(t)
	w := postGrowth(r, "/api/v1/growth/events", map[string]interface{}{
		"event_id": "xd-1", "kind": "safe_explore",
	}, "phone", "acc-99")
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	// other device same account
	wr := getGrowth(r, "/api/v1/growth/researcher", "tablet", "acc-99")
	assert.Equal(t, 200, wr.Code)
	var research map[string]interface{}
	require.NoError(t, json.Unmarshal(wr.Body.Bytes(), &research))
	assert.Equal(t, "acc:acc-99", research["owner_key"])
	tracks := research["tracks"].([]interface{})
	found := false
	for _, raw := range tracks {
		tr := raw.(map[string]interface{})
		if tr["track"] == models.GrowthTrackSafeObservation {
			assert.Equal(t, float64(5), tr["xp"])
			found = true
		}
	}
	assert.True(t, found)

	we := getGrowth(r, "/api/v1/growth/events", "tablet", "acc-99")
	assert.Equal(t, 200, we.Code)
	var ev map[string]interface{}
	require.NoError(t, json.Unmarshal(we.Body.Bytes(), &ev))
	assert.NotEmpty(t, ev["events"])
}

func TestGrowthHandler_ResetRequiresReason(t *testing.T) {
	r, _, _ := setupGrowthHandler(t)
	w := postGrowth(r, "/api/v1/growth/reset", map[string]interface{}{
		"scope": "all",
	}, "dev-1", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
