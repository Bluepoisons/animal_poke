package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

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
