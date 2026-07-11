package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"animalpoke/backend/internal/middleware"
	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/repo"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupQuestHandler(t *testing.T) (*gin.Engine, *repo.QuestRepo, *repo.WalletRepo) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dsn := fmt.Sprintf("file:%s/qh.db?_busy_timeout=5000&_journal_mode=WAL&cache=shared", t.TempDir())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(10)
	require.NoError(t, db.AutoMigrate(
		&models.WalletBalance{}, &models.WalletLedgerEntry{}, &models.InventoryItem{},
		&models.QuestDefinition{}, &models.QuestProgress{}, &models.QuestClaim{}, &models.QuestEventLog{},
	))
	wrepo := repo.NewWalletRepo(db)
	qrepo := repo.NewQuestRepo(db, wrepo)
	require.NoError(t, qrepo.SeedDefinitions())
	h := NewQuestHandler(qrepo)
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
		auth.GET("/quests", h.ListQuests)
		auth.GET("/quests/catalog", h.Catalog)
		auth.GET("/quests/:quest_id", h.GetQuest)
		auth.POST("/quests/events", h.ApplyEvents)
		auth.POST("/quests/:quest_id/claim", h.Claim)
		auth.POST("/quests/compensate", h.Compensate)
	}
	return r, qrepo, wrepo
}

func postQuest(r *gin.Engine, path string, body map[string]interface{}, device string) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if device != "" {
		req.Header.Set("X-Test-Device", device)
	}
	r.ServeHTTP(w, req)
	return w
}

func getQuest(r *gin.Engine, path, device string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, path, nil)
	if device != "" {
		req.Header.Set("X-Test-Device", device)
	}
	r.ServeHTTP(w, req)
	return w
}

func TestQuestHandler_CatalogMin24(t *testing.T) {
	r, _, _ := setupQuestHandler(t)
	w := getQuest(r, "/api/v1/quests/catalog", "dev-1")
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.GreaterOrEqual(t, int(resp["count"].(float64)), 24)
}

func TestQuestHandler_RejectPageView(t *testing.T) {
	r, _, _ := setupQuestHandler(t)
	w := postQuest(r, "/api/v1/quests/events", map[string]interface{}{
		"event_id": "pv1", "event_type": "open_pokedex",
	}, "dev-1")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "quest_event_forbidden", resp["reason_code"])
}

func TestQuestHandler_EventClaimIdempotent(t *testing.T) {
	r, _, _ := setupQuestHandler(t)
	w := postQuest(r, "/api/v1/quests/events", map[string]interface{}{
		"event_id": "cap-h1", "event_type": "capture_success",
	}, "dev-h")
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	// 幂等 event
	w2 := postQuest(r, "/api/v1/quests/events", map[string]interface{}{
		"event_id": "cap-h1", "event_type": "capture_success",
	}, "dev-h")
	assert.Equal(t, http.StatusOK, w2.Code)
	var er map[string]interface{}
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &er))
	assert.Equal(t, true, er["idempotent"])

	c1 := postQuest(r, "/api/v1/quests/main_first_capture/claim", map[string]interface{}{}, "dev-h")
	assert.Equal(t, http.StatusCreated, c1.Code, c1.Body.String())
	c2 := postQuest(r, "/api/v1/quests/main_first_capture/claim", map[string]interface{}{}, "dev-h")
	assert.Equal(t, http.StatusOK, c2.Code)
	var cr map[string]interface{}
	require.NoError(t, json.Unmarshal(c2.Body.Bytes(), &cr))
	assert.Equal(t, true, cr["idempotent"])
	assert.Equal(t, float64(30), cr["gold"])
}

func TestQuestHandler_ConcurrentClaim(t *testing.T) {
	r, _, wr := setupQuestHandler(t)
	// complete free daily checkin
	w := postQuest(r, "/api/v1/quests/events", map[string]interface{}{
		"event_id": "chk-c1", "event_type": "season_checkin",
	}, "dev-cc")
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	const n = 24
	var wg sync.WaitGroup
	var created, idem atomic.Int64
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			resp := postQuest(r, "/api/v1/quests/daily_season_checkin/claim", map[string]interface{}{}, "dev-cc")
			if resp.Code == http.StatusCreated {
				created.Add(1)
			} else if resp.Code == http.StatusOK {
				idem.Add(1)
			} else {
				t.Errorf("unexpected %d %s", resp.Code, resp.Body.String())
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, int64(n), created.Load()+idem.Load())
	assert.LessOrEqual(t, created.Load(), int64(1))
	// 金币只入账一次（幂等）
	bals, err := wr.GetBalances("", "dev-cc")
	require.NoError(t, err)
	var gold int64
	for _, b := range bals {
		if b.Currency == models.CurrencyGold {
			gold = b.Balance
		}
	}
	assert.Equal(t, int64(8), gold)
}

func TestQuestHandler_ListFree(t *testing.T) {
	r, _, _ := setupQuestHandler(t)
	w := getQuest(r, "/api/v1/quests?free=1", "dev-f")
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	quests := resp["quests"].([]interface{})
	assert.NotEmpty(t, quests)
	for _, q := range quests {
		m := q.(map[string]interface{})
		assert.Equal(t, true, m["free"])
	}
}

func TestQuestHandler_ListAndGet(t *testing.T) {
	r, _, _ := setupQuestHandler(t)
	w := getQuest(r, "/api/v1/quests", "dev-l")
	assert.Equal(t, http.StatusOK, w.Code)
	w2 := getQuest(r, "/api/v1/quests/main_first_capture", "dev-l")
	assert.Equal(t, http.StatusOK, w2.Code, w2.Body.String())
}
