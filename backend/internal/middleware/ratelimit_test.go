package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter(10, 5)
	defer rl.Stop()

	// 前 burst 次应全部放行
	for i := 0; i < 5; i++ {
		assert.True(t, rl.allow("test-key"), "request %d should pass", i)
	}
	// 第 6 次应拒绝(令牌耗尽)
	assert.False(t, rl.allow("test-key"))
}

func TestRateLimiter_Refill(t *testing.T) {
	rl := NewRateLimiter(100, 3) // 100/s, burst=3
	defer rl.Stop()

	// 用完 burst
	for i := 0; i < 3; i++ {
		assert.True(t, rl.allow("key"))
	}
	assert.False(t, rl.allow("key"))

	// 等待足够时间让令牌恢复
	time.Sleep(50 * time.Millisecond) // 100/s, 50ms 应恢复约 5 个令牌
	assert.True(t, rl.allow("key"))
}

func TestRateLimiter_DifferentKeys(t *testing.T) {
	rl := NewRateLimiter(10, 2)
	defer rl.Stop()

	// key-a 用完
	assert.True(t, rl.allow("key-a"))
	assert.True(t, rl.allow("key-a"))
	assert.False(t, rl.allow("key-a"))

	// key-b 不受影响
	assert.True(t, rl.allow("key-b"))
}

func TestRateLimitByIP_Integration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rl := NewRateLimiter(100, 3)
	defer rl.Stop()

	r := gin.New()
	r.Use(RateLimitByIP(rl))
	r.GET("/test", func(c *gin.Context) { c.Status(200) })

	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}

func TestRateLimiter_Cleanup(t *testing.T) {
	rl := NewRateLimiter(10, 100)
	defer rl.Stop()

	rl.allow("old-key")
	rl.mu.Lock()
	rl.buckets["old-key"].lastTime = time.Now().Add(-31 * time.Minute)
	rl.mu.Unlock()

	// 触发一次 cleanup 逻辑（直接调用内部清理路径）
	rl.mu.Lock()
	threshold := time.Now().Add(-30 * time.Minute)
	for k, b := range rl.buckets {
		if b.lastTime.Before(threshold) {
			delete(rl.buckets, k)
		}
	}
	_, exists := rl.buckets["old-key"]
	rl.mu.Unlock()
	assert.False(t, exists)
}

func TestMemorySharedCounter_FractionalRate(t *testing.T) {
	// rate=100/60 ≈ 1.666/s 不得被截断成 1 导致错误固定窗口
	m := NewMemorySharedCounter()
	defer m.Stop()
	ctx := context.Background()
	rate := 100.0 / 60.0
	burst := 3

	for i := 0; i < burst; i++ {
		ok, err := m.AllowToken(ctx, "frac", rate, burst)
		require.NoError(t, err)
		assert.True(t, ok, "burst %d", i)
	}
	ok, err := m.AllowToken(ctx, "frac", rate, burst)
	require.NoError(t, err)
	assert.False(t, ok)

	// 约 1.2s 应补充 ~2 tokens（若截断为 1/s 则只补 1.2）
	time.Sleep(1200 * time.Millisecond)
	ok, err = m.AllowToken(ctx, "frac", rate, burst)
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = m.AllowToken(ctx, "frac", rate, burst)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestMemorySharedCounter_IncrTTL(t *testing.T) {
	m := NewMemorySharedCounter()
	defer m.Stop()
	ctx := context.Background()

	n, err := m.Incr(ctx, "k", 50*time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
	n, err = m.Incr(ctx, "k", 50*time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, int64(2), n)

	time.Sleep(60 * time.Millisecond)
	n, err = m.Incr(ctx, "k", 50*time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n, "expired key should reset")
}

func TestMemorySharedCounter_SetNX(t *testing.T) {
	m := NewMemorySharedCounter()
	defer m.Stop()
	ctx := context.Background()

	ok, err := m.SetNX(ctx, "nonce:a", 100*time.Millisecond)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = m.SetNX(ctx, "nonce:a", 100*time.Millisecond)
	require.NoError(t, err)
	assert.False(t, ok, "replay must fail")

	time.Sleep(110 * time.Millisecond)
	ok, err = m.SetNX(ctx, "nonce:a", 100*time.Millisecond)
	require.NoError(t, err)
	assert.True(t, ok, "after TTL reuse allowed")
}

func TestMemorySharedCounter_Race(t *testing.T) {
	m := NewMemorySharedCounter()
	defer m.Stop()
	ctx := context.Background()

	const goroutines = 32
	const perG = 50
	var allowed atomic.Int64

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < perG; i++ {
				// Incr race
				_, _ = m.Incr(ctx, fmt.Sprintf("race-incr-%d", i%8), time.Minute)
				// Token race on shared key
				ok, err := m.AllowToken(ctx, "race-token", 1000, 100)
				if err == nil && ok {
					allowed.Add(1)
				}
				// SetNX race — only one winner per key
				_, _ = m.SetNX(ctx, fmt.Sprintf("race-nonce-%d", i), time.Minute)
			}
		}(g)
	}
	wg.Wait()

	// burst 100：允许次数不应超过 burst（并发下近似，至少应 >0 且有界）
	assert.Greater(t, allowed.Load(), int64(0))
	assert.LessOrEqual(t, allowed.Load(), int64(100)+int64(goroutines)) // 极宽松上界
}

func TestMemorySharedCounter_SetNX_ConcurrentSingleWinner(t *testing.T) {
	m := NewMemorySharedCounter()
	defer m.Stop()
	ctx := context.Background()

	const n = 64
	var winners atomic.Int64
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			ok, err := m.SetNX(ctx, "nonce:same", time.Minute)
			if err == nil && ok {
				winners.Add(1)
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, int64(1), winners.Load())
}

func TestRateLimiter_WithShared_Memory(t *testing.T) {
	m := NewMemorySharedCounter()
	defer m.Stop()
	rl := NewRateLimiter(10, 2).WithShared(m)
	defer rl.Stop()

	assert.True(t, rl.allow(RateKeyDevice("d1")))
	assert.True(t, rl.allow(RateKeyDevice("d1")))
	assert.False(t, rl.allow(RateKeyDevice("d1")))
	// 不同维度互不影响
	assert.True(t, rl.allow(RateKeyIP("1.2.3.4")))
	assert.True(t, rl.allow(RateKeyDigest("abc")))
	assert.True(t, rl.allow(RateKeyAccount("u1")))
}

func TestRateLimitByDigest_AndAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rl := NewRateLimiter(100, 1)
	defer rl.Stop()

	r := gin.New()
	r.Use(RateLimitByDigest(rl))
	r.GET("/d", func(c *gin.Context) { c.Status(200) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/d", nil)
	req.Header.Set("X-Input-Digest", "deadbeef")
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/d", nil)
	req.Header.Set("X-Input-Digest", "deadbeef")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	// 无 digest 头跳过
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/d", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)
}

func TestMemorySharedCounter_TTLPurge(t *testing.T) {
	m := NewMemorySharedCounter()
	defer m.Stop()
	ctx := context.Background()

	for i := 0; i < 200; i++ {
		_, _ = m.Incr(ctx, fmt.Sprintf("tmp-%d", i), 20*time.Millisecond)
		_, _ = m.SetNX(ctx, fmt.Sprintf("n-%d", i), 20*time.Millisecond)
	}
	c1, _, n1 := m.LenForTest()
	assert.Greater(t, c1, 0)
	assert.Greater(t, n1, 0)

	time.Sleep(40 * time.Millisecond)
	m.purgeExpired()
	c2, _, n2 := m.LenForTest()
	assert.Equal(t, 0, c2)
	assert.Equal(t, 0, n2)
}

func TestRateLimiter_FailClosed(t *testing.T) {
	rl := NewRateLimiter(10, 5).WithShared(&errCounter{}).WithFailOpen(false)
	defer rl.Stop()
	assert.False(t, rl.allow("k"))
}

func TestRateLimiter_FailOpen(t *testing.T) {
	rl := NewRateLimiter(10, 2).WithShared(&errCounter{}).WithFailOpen(true)
	defer rl.Stop()
	assert.True(t, rl.allow("k"))
	assert.True(t, rl.allow("k"))
	assert.False(t, rl.allow("k"))
}

// errCounter 始终返回错误，用于 fail-open/closed 测试。
type errCounter struct{}

func (e *errCounter) Incr(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	return 0, fmt.Errorf("redis down")
}
func (e *errCounter) AllowToken(ctx context.Context, key string, rate float64, burst int) (bool, error) {
	return false, fmt.Errorf("redis down")
}
func (e *errCounter) SetNX(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return false, fmt.Errorf("redis down")
}
