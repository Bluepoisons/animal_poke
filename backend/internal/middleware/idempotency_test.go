package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"animalpoke/backend/internal/models"
	"animalpoke/backend/internal/repo"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupIdemRouter(t *testing.T) (*gin.Engine, *int64) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:idem_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.IdempotencyRecord{}))
	store := repo.NewIdempotencyRepo(db)

	var hits int64
	r := gin.New()
	r.POST("/api/v1/value/generate", func(c *gin.Context) {
		c.Set("device_id", "dev-1")
	}, Idempotency(store, "value.generate"), func(c *gin.Context) {
		atomic.AddInt64(&hits, 1)
		c.JSON(http.StatusOK, gin.H{"ok": true, "n": atomic.LoadInt64(&hits)})
	})
	return r, &hits
}

func TestIdempotency_ReplaySameKey(t *testing.T) {
	r, hits := setupIdemRouter(t)
	body := []byte(`{"species":"cat"}`)
	do := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/value/generate", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "k1")
		r.ServeHTTP(w, req)
		return w
	}
	w1 := do()
	assert.Equal(t, 200, w1.Code)
	w2 := do()
	assert.Equal(t, 200, w2.Code)
	assert.Equal(t, "true", w2.Header().Get("X-Idempotency-Replayed"))
	assert.Equal(t, int64(1), atomic.LoadInt64(hits))
	assert.JSONEq(t, w1.Body.String(), w2.Body.String())
}

func TestIdempotency_ConflictDifferentBody(t *testing.T) {
	r, _ := setupIdemRouter(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/value/generate", bytes.NewReader([]byte(`{"species":"cat"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "k2")
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/api/v1/value/generate", bytes.NewReader([]byte(`{"species":"dog"}`)))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", "k2")
	r.ServeHTTP(w2, req2)
	assert.Equal(t, 409, w2.Code)
	assert.Contains(t, w2.Body.String(), "idempotency_conflict")
}

func TestIdempotency_DoesNotCacheQuotaResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:idem_quota_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.IdempotencyRecord{}))

	var hits int64
	r := gin.New()
	r.POST("/api/v1/adventures", func(c *gin.Context) {
		c.Set("device_id", "dev-1")
	}, Idempotency(repo.NewIdempotencyRepo(db), "adventure.generate"), func(c *gin.Context) {
		n := atomic.AddInt64(&hits, 1)
		if n == 1 {
			c.JSON(http.StatusTooManyRequests, gin.H{"reason_code": "daily_cost_limit"})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"ok": true})
	})

	do := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/adventures", bytes.NewBufferString(`{"operation_id":"quota-retry"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "quota-retry")
		r.ServeHTTP(w, req)
		return w
	}
	assert.Equal(t, http.StatusTooManyRequests, do().Code)
	second := do()
	assert.Equal(t, http.StatusCreated, second.Code)
	assert.Empty(t, second.Header().Get("X-Idempotency-Replayed"))
	assert.Equal(t, int64(2), atomic.LoadInt64(&hits))
}

func TestIdempotency_ServerFailureCanRetryImmediately(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:idem_failure_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.IdempotencyRecord{}))

	var hits int64
	r := gin.New()
	r.POST("/api/v1/adventures", func(c *gin.Context) {
		c.Set("device_id", "dev-1")
	}, Idempotency(repo.NewIdempotencyRepo(db), "adventure.generate"), func(c *gin.Context) {
		if atomic.AddInt64(&hits, 1) == 1 {
			c.JSON(http.StatusGatewayTimeout, gin.H{"reason_code": "upstream_timeout"})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"ok": true})
	})

	do := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/adventures", bytes.NewBufferString(`{"operation_id":"timeout-retry"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(HeaderIdempotencyKey, "timeout-retry")
		r.ServeHTTP(w, req)
		return w
	}
	assert.Equal(t, http.StatusGatewayTimeout, do().Code)
	assert.Equal(t, http.StatusCreated, do().Code)
	assert.Equal(t, int64(2), atomic.LoadInt64(&hits))
}

func TestIdempotency_ConcurrentSingleExecution(t *testing.T) {
	r, hits := setupIdemRouter(t)
	var wg sync.WaitGroup
	n := 20
	wg.Add(n)
	codes := make([]int, n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/v1/value/generate", bytes.NewReader([]byte(`{"species":"cat"}`)))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Idempotency-Key", "concurrent-1")
			r.ServeHTTP(w, req)
			codes[i] = w.Code
		}(i)
	}
	wg.Wait()
	// under concurrency: at most one successful handler execution for same key+body
	assert.LessOrEqual(t, atomic.LoadInt64(hits), int64(1))
	// sqlite may return 500 on lock; production MySQL unique index is the hard gate
	for _, c := range codes {
		assert.True(t, c == 200 || c == 409 || c == 500, "code=%d", c)
	}
	_ = json.Marshal
}

func TestIdempotency_ConcurrentStaleTakeoverSingleExecution(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:idem_stale_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&models.IdempotencyRecord{}))
	store := repo.NewIdempotencyRepo(db)
	body := []byte(`{"species":"cat"}`)
	staleAt := time.Now().UTC().Add(-ProcessingTimeout - time.Minute)
	require.NoError(t, db.Create(&models.IdempotencyRecord{
		DeviceID:    "dev-stale",
		Route:       "value.generate",
		Key:         "stale-key",
		RequestHash: hashRequest(http.MethodPost, "/api/v1/value/generate", body),
		Status:      "processing",
		CreatedAt:   staleAt,
		UpdatedAt:   staleAt,
		ExpiresAt:   time.Now().UTC().Add(time.Hour),
	}).Error)

	var hits int64
	started := make(chan struct{})
	release := make(chan struct{})
	var startedOnce sync.Once
	r := gin.New()
	r.POST("/api/v1/value/generate", func(c *gin.Context) {
		c.Set("device_id", "dev-stale")
	}, Idempotency(store, "value.generate"), func(c *gin.Context) {
		atomic.AddInt64(&hits, 1)
		startedOnce.Do(func() { close(started) })
		<-release
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	const requests = 12
	launch := make(chan struct{})
	done := make(chan int, requests)
	responses := make([]*httptest.ResponseRecorder, requests)
	for i := 0; i < requests; i++ {
		go func(index int) {
			<-launch
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/value/generate", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set(HeaderIdempotencyKey, "stale-key")
			r.ServeHTTP(w, req)
			responses[index] = w
			done <- index
		}(i)
	}
	close(launch)
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("stale takeover did not reach handler")
	}

	for i := 0; i < requests-1; i++ {
		select {
		case index := <-done:
			assert.Equal(t, http.StatusConflict, responses[index].Code, responses[index].Body.String())
			assert.Contains(t, responses[index].Body.String(), "idempotency_in_progress")
		case <-time.After(3 * time.Second):
			t.Fatal("concurrent stale loser did not return")
		}
	}
	close(release)
	winner := <-done
	assert.Equal(t, http.StatusOK, responses[winner].Code, responses[winner].Body.String())
	assert.Equal(t, int64(1), atomic.LoadInt64(&hits))
}

func TestIdempotency_StaleLeaseCannotOverwriteTakeover(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:idem_fence_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.IdempotencyRecord{}))
	store := repo.NewIdempotencyRepo(db)
	staleAt := time.Now().UTC().Add(-ProcessingTimeout - time.Minute)
	stale := &models.IdempotencyRecord{
		DeviceID: "dev-fence", Route: "value.generate", Key: "fence-key", RequestHash: "hash",
		Status: "processing", CreatedAt: staleAt, UpdatedAt: staleAt, ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	require.NoError(t, db.Create(stale).Error)
	claim, won, err := store.TryTakeover(stale, stale.RequestHash, DefaultIdempotencyTTL)
	require.NoError(t, err)
	require.True(t, won)

	completed, err := store.CompleteClaim(stale, http.StatusOK, `{"worker":"stale"}`, true)
	require.NoError(t, err)
	assert.False(t, completed)
	completed, err = store.CompleteClaim(claim, http.StatusCreated, `{"worker":"winner"}`, true)
	require.NoError(t, err)
	assert.True(t, completed)

	var saved models.IdempotencyRecord
	require.NoError(t, db.First(&saved, claim.ID).Error)
	assert.Equal(t, "completed", saved.Status)
	assert.Equal(t, http.StatusCreated, saved.HTTPStatus)
	assert.JSONEq(t, `{"worker":"winner"}`, saved.ResponseBody)
}
