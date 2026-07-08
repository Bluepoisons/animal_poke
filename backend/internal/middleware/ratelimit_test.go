package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
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
		req, _ := http.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		r.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code, "request %d should pass", i)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	r.ServeHTTP(w, req)
	assert.Equal(t, 429, w.Code)
}

func TestRateLimiter_Cleanup(t *testing.T) {
	rl := NewRateLimiter(10, 100)
	defer rl.Stop()

	rl.allow("old-key")
	rl.mu.Lock()
	b := rl.buckets["old-key"]
	b.lastTime = time.Now().Add(-2 * time.Hour) // 设为过期
	rl.mu.Unlock()

	// 手动触发一次清理(cleanup goroutine 每 10 分钟触发的逻辑, 这里直接调)
	rl.mu.Lock()
	threshold := time.Now().Add(-30 * time.Minute)
	for k, b := range rl.buckets {
		if b.lastTime.Before(threshold) {
			delete(rl.buckets, k)
		}
	}
	rl.mu.Unlock()

	_, ok := rl.buckets["old-key"]
	assert.False(t, ok, "过期 bucket 应被清理")
}
