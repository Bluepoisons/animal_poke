// Package middleware 简单令牌桶限流中间件(内存实现)。
package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter 基于令牌桶的内存限流器。
type RateLimiter struct {
	rate     float64 // 每秒允许请求数
	burst    int     // 突发允许量
	buckets  map[string]*tokenBucket
	mu       sync.Mutex
	cleanupT *time.Ticker
}

type tokenBucket struct {
	tokens   float64
	lastTime time.Time
}

// NewRateLimiter rate 每秒允许请求数, burst 突发容量。
func NewRateLimiter(rate float64, burst int) *RateLimiter {
	rl := &RateLimiter{
		rate:     rate,
		burst:    burst,
		buckets:  make(map[string]*tokenBucket),
		cleanupT: time.NewTicker(10 * time.Minute),
	}
	go rl.cleanup()
	return rl
}

// cleanup 定期清理过期的 bucket。
func (rl *RateLimiter) cleanup() {
	for range rl.cleanupT.C {
		rl.mu.Lock()
		threshold := time.Now().Add(-30 * time.Minute)
		for k, b := range rl.buckets {
			if b.lastTime.Before(threshold) {
				delete(rl.buckets, k)
			}
		}
		rl.mu.Unlock()
	}
}

// allow 检查 key 是否允许本次请求。
func (rl *RateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, ok := rl.buckets[key]
	now := time.Now()
	if !ok {
		rl.buckets[key] = &tokenBucket{tokens: float64(rl.burst) - 1, lastTime: now}
		return true
	}

	elapsed := now.Sub(b.lastTime).Seconds()
	b.tokens += elapsed * rl.rate
	if b.tokens > float64(rl.burst) {
		b.tokens = float64(rl.burst)
	}
	b.lastTime = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// RateLimitByDevice 按 device_id 限流(需在 JWTAuth 之后使用)。
func RateLimitByDevice(rl *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		deviceID := GetDeviceID(c)
		if deviceID == "" {
			c.Next()
			return
		}
		if !rl.allow(deviceID) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}

// RateLimitByIP 按客户端 IP 限流(无需鉴权)。
func RateLimitByIP(rl *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !rl.allow(c.ClientIP()) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}

// Stop 停止清理 goroutine。
func (rl *RateLimiter) Stop() {
	rl.cleanupT.Stop()
}
