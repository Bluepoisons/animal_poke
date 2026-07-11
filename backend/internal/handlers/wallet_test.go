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

func setupWalletHandler(t *testing.T) (*gin.Engine, *repo.WalletRepo) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dsn := fmt.Sprintf("file:%s/wh.db?_busy_timeout=5000&_journal_mode=WAL&cache=shared", t.TempDir())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(10)
	require.NoError(t, db.AutoMigrate(
		&models.WalletBalance{}, &models.WalletLedgerEntry{}, &models.InventoryItem{},
	))
	wrepo := repo.NewWalletRepo(db)
	h := NewWalletHandler(wrepo)
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
		auth.GET("/wallet", h.GetWallet)
		auth.POST("/wallet/credit", h.Credit)
		auth.POST("/wallet/debit", h.Debit)
		auth.GET("/wallet/ledger", h.ListLedger)
		auth.POST("/wallet/reconcile", h.Reconcile)
		auth.GET("/inventory", h.GetInventory)
		auth.POST("/inventory/grant", h.GrantInventory)
		auth.POST("/inventory/consume", h.ConsumeInventory)
	}
	return r, wrepo
}

func postWallet(r *gin.Engine, path string, body map[string]interface{}, device, account string) *httptest.ResponseRecorder {
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

func getWallet(r *gin.Engine, path, device, account string) *httptest.ResponseRecorder {
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

func TestWalletHandler_CreditIdempotent(t *testing.T) {
	r, _ := setupWalletHandler(t)
	body := map[string]interface{}{
		"currency": "gold", "amount": 80, "operation_id": "h-op-1",
		"source_type": "checkin", "source_id": "d1",
	}
	w1 := postWallet(r, "/api/v1/wallet/credit", body, "dev-1", "")
	assert.Equal(t, http.StatusCreated, w1.Code, w1.Body.String())
	var resp1 map[string]interface{}
	require.NoError(t, json.Unmarshal(w1.Body.Bytes(), &resp1))
	assert.Equal(t, float64(80), resp1["balance"])
	assert.Equal(t, false, resp1["idempotent"])

	w2 := postWallet(r, "/api/v1/wallet/credit", body, "dev-1", "")
	assert.Equal(t, http.StatusOK, w2.Code)
	var resp2 map[string]interface{}
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp2))
	assert.Equal(t, float64(80), resp2["balance"])
	assert.Equal(t, true, resp2["idempotent"])
}

func TestWalletHandler_DebitInsufficient(t *testing.T) {
	r, _ := setupWalletHandler(t)
	w := postWallet(r, "/api/v1/wallet/debit", map[string]interface{}{
		"currency": "gold", "amount": 10, "operation_id": "h-d1", "source_type": "shop",
	}, "dev-1", "")
	assert.Equal(t, http.StatusConflict, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "insufficient_balance", resp["reason_code"])
}

func TestWalletHandler_GetWalletAndLedger(t *testing.T) {
	r, _ := setupWalletHandler(t)
	postWallet(r, "/api/v1/wallet/credit", map[string]interface{}{
		"currency": "gold", "amount": 40, "operation_id": "g1", "source_type": "reward",
	}, "dev-x", "")
	postWallet(r, "/api/v1/wallet/debit", map[string]interface{}{
		"currency": "gold", "amount": 15, "operation_id": "g2", "source_type": "shop",
	}, "dev-x", "")

	w := getWallet(r, "/api/v1/wallet", "dev-x", "")
	assert.Equal(t, http.StatusOK, w.Code)
	var wallet map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &wallet))
	balances := wallet["balances"].([]interface{})
	found := false
	for _, b := range balances {
		m := b.(map[string]interface{})
		if m["currency"] == "gold" {
			assert.Equal(t, float64(25), m["balance"])
			found = true
		}
	}
	assert.True(t, found)

	wl := getWallet(r, "/api/v1/wallet/ledger?currency=gold&limit=10", "dev-x", "")
	assert.Equal(t, http.StatusOK, wl.Code)
	var led map[string]interface{}
	require.NoError(t, json.Unmarshal(wl.Body.Bytes(), &led))
	entries := led["entries"].([]interface{})
	assert.GreaterOrEqual(t, len(entries), 2)
}

func TestWalletHandler_ConcurrentDebit(t *testing.T) {
	r, _ := setupWalletHandler(t)
	// 种子 100
	w := postWallet(r, "/api/v1/wallet/credit", map[string]interface{}{
		"currency": "gold", "amount": 100, "operation_id": "seed", "source_type": "admin",
	}, "dev-c", "")
	require.Equal(t, http.StatusCreated, w.Code)

	const n = 40
	var wg sync.WaitGroup
	var okCount atomic.Int64
	var failCount atomic.Int64
	wg.Add(n)
	for i := range n {
		i := i
		go func() {
			defer wg.Done()
			resp := postWallet(r, "/api/v1/wallet/debit", map[string]interface{}{
				"currency": "gold", "amount": 5, "operation_id": fmt.Sprintf("cd-%d", i), "source_type": "shop",
			}, "dev-c", "")
			if resp.Code == http.StatusOK {
				okCount.Add(1)
			} else if resp.Code == http.StatusConflict {
				failCount.Add(1)
			} else {
				t.Errorf("unexpected status %d: %s", resp.Code, resp.Body.String())
			}
		}()
	}
	wg.Wait()
	// 100/5 = 20 成功
	assert.Equal(t, int64(20), okCount.Load())
	assert.Equal(t, int64(20), failCount.Load())

	gw := getWallet(r, "/api/v1/wallet", "dev-c", "")
	var wallet map[string]interface{}
	require.NoError(t, json.Unmarshal(gw.Body.Bytes(), &wallet))
	for _, b := range wallet["balances"].([]interface{}) {
		m := b.(map[string]interface{})
		if m["currency"] == "gold" {
			assert.Equal(t, float64(0), m["balance"])
		}
	}
}

func TestWalletHandler_Reconcile(t *testing.T) {
	r, wr := setupWalletHandler(t)
	postWallet(r, "/api/v1/wallet/credit", map[string]interface{}{
		"currency": "stamina", "amount": 50, "operation_id": "st1", "source_type": "system",
	}, "dev-r", "")
	postWallet(r, "/api/v1/wallet/debit", map[string]interface{}{
		"currency": "stamina", "amount": 20, "operation_id": "st2", "source_type": "capture",
	}, "dev-r", "")

	// 破坏快照
	require.NoError(t, wr.DB().Model(&models.WalletBalance{}).
		Where("owner_key = ?", repo.OwnerKey("", "dev-r")).
		Update("balance", 0).Error)

	w := postWallet(r, "/api/v1/wallet/reconcile", map[string]interface{}{
		"currency": "stamina",
	}, "dev-r", "")
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(30), resp["balance"])
}

func TestWalletHandler_Inventory(t *testing.T) {
	r, _ := setupWalletHandler(t)
	w := postWallet(r, "/api/v1/inventory/grant", map[string]interface{}{
		"item_id": "toy-ball", "quantity": 3, "operation_id": "ig1", "source_type": "checkin",
	}, "dev-i", "")
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	// 幂等
	w2 := postWallet(r, "/api/v1/inventory/grant", map[string]interface{}{
		"item_id": "toy-ball", "quantity": 3, "operation_id": "ig1", "source_type": "checkin",
	}, "dev-i", "")
	assert.Equal(t, http.StatusOK, w2.Code)
	var g map[string]interface{}
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &g))
	assert.Equal(t, true, g["idempotent"])

	w3 := postWallet(r, "/api/v1/inventory/consume", map[string]interface{}{
		"item_id": "toy-ball", "quantity": 1, "operation_id": "ic1", "source_type": "capture",
	}, "dev-i", "")
	assert.Equal(t, http.StatusOK, w3.Code)

	list := getWallet(r, "/api/v1/inventory", "dev-i", "")
	assert.Equal(t, http.StatusOK, list.Code)
	var inv map[string]interface{}
	require.NoError(t, json.Unmarshal(list.Body.Bytes(), &inv))
	items := inv["items"].([]interface{})
	require.Len(t, items, 1)
	assert.Equal(t, float64(2), items[0].(map[string]interface{})["quantity"])
}

func TestWalletHandler_InvalidCurrency(t *testing.T) {
	r, _ := setupWalletHandler(t)
	w := postWallet(r, "/api/v1/wallet/credit", map[string]interface{}{
		"currency": "diamonds", "amount": 1, "operation_id": "bad-c", "source_type": "admin",
	}, "dev-1", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
