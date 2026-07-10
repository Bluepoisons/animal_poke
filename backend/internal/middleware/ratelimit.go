// Package middleware 限流：内存实现 + 可选 Redis 共享存储接口。
package middleware

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// SharedCounter 跨 Pod 共享计数接口（Redis 等）。
type SharedCounter interface {
	// Incr 原子自增并返回新值；ttl 在 key 不存在时设置。
	Incr(ctx context.Context, key string, ttl time.Duration) (int64, error)
	// AllowToken 令牌桶风格：返回是否允许。
	AllowToken(ctx context.Context, key string, rate float64, burst int) (bool, error)
}

// RateLimiter 基于令牌桶的限流器（内存默认；可注入 SharedCounter）。
type RateLimiter struct {
	rate     float64
	burst    int
	buckets  map[string]*tokenBucket
	mu       sync.Mutex
	cleanupT *time.Ticker
	shared   SharedCounter
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

// WithShared 注入共享存储。
func (rl *RateLimiter) WithShared(s SharedCounter) *RateLimiter {
	rl.shared = s
	return rl
}

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

func (rl *RateLimiter) allow(key string) bool {
	if rl.shared != nil {
		ok, err := rl.shared.AllowToken(context.Background(), "rl:"+key, rl.rate, rl.burst)
		if err == nil {
			return ok
		}
		// 共享存储失败时降级本地
	}
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

// RateLimitByDevice 按 device_id 限流。
func RateLimitByDevice(rl *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		deviceID := GetDeviceID(c)
		if deviceID == "" {
			c.Next()
			return
		}
		if !rl.allow(deviceID) {
			AbortTooMany(c, "rate_limited", "rate limit exceeded", 1, nil)
			return
		}
		c.Next()
	}
}

// RateLimitByIP 按客户端 IP 限流。
func RateLimitByIP(rl *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !rl.allow("ip:" + c.ClientIP()) {
			AbortTooMany(c, "rate_limited", "rate limit exceeded", 1, nil)
			return
		}
		c.Next()
	}
}

// Stop 停止清理 goroutine。
func (rl *RateLimiter) Stop() {
	rl.cleanupT.Stop()
}

// MemorySharedCounter 进程内共享计数（单测 / 单 Pod 兜底，接口兼容 Redis）。
type MemorySharedCounter struct {
	mu   sync.Mutex
	data map[string]*memCount
}

type memCount struct {
	n       int64
	expires time.Time
}

// NewMemorySharedCounter 构造。
func NewMemorySharedCounter() *MemorySharedCounter {
	return &MemorySharedCounter{data: make(map[string]*memCount)}
}

// Incr 实现 SharedCounter。
func (m *MemorySharedCounter) Incr(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	c, ok := m.data[key]
	if !ok || now.After(c.expires) {
		c = &memCount{n: 0, expires: now.Add(ttl)}
		m.data[key] = c
	}
	c.n++
	return c.n, nil
}

// AllowToken 简易实现。
func (m *MemorySharedCounter) AllowToken(ctx context.Context, key string, rate float64, burst int) (bool, error) {
	// 用每秒窗口近似
	n, err := m.Incr(ctx, key+":"+strconv.FormatInt(time.Now().Unix(), 10), 2*time.Second)
	if err != nil {
		return false, err
	}
	limit := int64(rate) + int64(burst)
	if limit < 1 {
		limit = 1
	}
	return n <= limit, nil
}
