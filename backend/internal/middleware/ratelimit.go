// Package middleware 限流：内存实现 + 可选 Redis 共享存储接口。
//
// Fail-open / fail-closed 策略（Redis 故障时）：
//   - RateLimitByIP（/auth/device）：fail-open — 共享存储错误时降级本机令牌桶，保证登录可用性。
//   - RateLimitByDevice（AI 识别类）：fail-open — 同上，避免 Redis 抖动导致全站 429。
//   - CostLimitByType（每日配额）：fail-open — Incr 失败时降级本机计数。
//   - SecurityHandler nonce（/security/report）：fail-closed — SetNX 失败返回 503，避免跨 Pod 重放穿透。
//
// 未配置 REDIS_URL 时全程使用 MemorySharedCounter（单 Pod 语义）。
package middleware

import (
	"context"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// SharedCounter 跨 Pod 共享计数 / 令牌桶 / nonce 接口（Redis 或内存）。
type SharedCounter interface {
	// Incr 原子自增并返回新值；ttl 仅在 key 首次创建时设置。
	Incr(ctx context.Context, key string, ttl time.Duration) (int64, error)
	// AllowToken 令牌桶：rate 为每秒补充令牌数（保留小数，不得截断为固定窗口），burst 为桶容量。
	AllowToken(ctx context.Context, key string, rate float64, burst int) (bool, error)
	// SetNX 仅当 key 不存在时写入并设置 TTL；true 表示首次占用成功。
	SetNX(ctx context.Context, key string, ttl time.Duration) (bool, error)
}

// 多维限流 key 前缀，保证 device / account / ip / digest 维度可组合且不冲突。
const (
	KeyPrefixDevice  = "device:"
	KeyPrefixAccount = "account:"
	KeyPrefixIP      = "ip:"
	KeyPrefixDigest  = "digest:"
	KeyPrefixNonce   = "nonce:"
	KeyPrefixRL      = "rl:"
	KeyPrefixCost    = "cost:"
)

// RateKeyDevice 设备维度限流 key。
func RateKeyDevice(id string) string { return KeyPrefixDevice + id }

// RateKeyAccount 账号维度限流 key。
func RateKeyAccount(id string) string { return KeyPrefixAccount + id }

// RateKeyIP IP 维度限流 key。
func RateKeyIP(ip string) string { return KeyPrefixIP + ip }

// RateKeyDigest 推理输入摘要维度限流 key。
func RateKeyDigest(digest string) string { return KeyPrefixDigest + digest }

// RateLimiter 基于令牌桶的限流器（内存默认；可注入 SharedCounter）。
type RateLimiter struct {
	rate     float64
	burst    int
	buckets  map[string]*tokenBucket
	mu       sync.Mutex
	cleanupT *time.Ticker
	shared   SharedCounter
	// failOpen 为 true 时共享存储错误降级本地（默认 true）。
	failOpen bool
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
		failOpen: true,
	}
	go rl.cleanup()
	return rl
}

// WithShared 注入共享存储。
func (rl *RateLimiter) WithShared(s SharedCounter) *RateLimiter {
	rl.shared = s
	return rl
}

// WithFailOpen 设置共享存储故障策略：true=降级本地，false=拒绝请求。
func (rl *RateLimiter) WithFailOpen(v bool) *RateLimiter {
	rl.failOpen = v
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
		ok, err := rl.shared.AllowToken(context.Background(), KeyPrefixRL+key, rl.rate, rl.burst)
		if err == nil {
			return ok
		}
		// 共享存储失败：fail-open 降级本地，fail-closed 拒绝。
		if !rl.failOpen {
			return false
		}
	}
	return rl.allowLocal(key)
}

func (rl *RateLimiter) allowLocal(key string) bool {
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
// Fail-open：共享存储错误时降级本机桶（见 package 注释）。
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
// Fail-open：共享存储错误时降级本机桶（见 package 注释）。
func RateLimitByIP(rl *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !rl.allow("ip:" + c.ClientIP()) {
			AbortTooMany(c, "rate_limited", "rate limit exceeded", 1, nil)
			return
		}
		c.Next()
	}
}

// RateLimitByAccount 按 account_id（从 context 读取，缺省跳过）限流。
func RateLimitByAccount(rl *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		accountID, _ := c.Get("account_id")
		id, _ := accountID.(string)
		if id == "" {
			c.Next()
			return
		}
		if !rl.allow(RateKeyAccount(id)) {
			ObserveRateLimit()
			AbortTooMany(c, "rate_limit_account", "rate limit exceeded", 1, nil)
			return
		}
		c.Next()
	}
}

// RateLimitByDigest 按请求头 X-Input-Digest（推理输入摘要）限流，防同图刷量。
func RateLimitByDigest(rl *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		digest := c.GetHeader("X-Input-Digest")
		if digest == "" {
			c.Next()
			return
		}
		if !rl.allow(RateKeyDigest(digest)) {
			ObserveRateLimit()
			AbortTooMany(c, "rate_limit_digest", "rate limit exceeded", 1, nil)
			return
		}
		c.Next()
	}
}

// RateLimitMulti 多维度串联限流：所有 dimension key 均需通过。
// keys 由调用方用 RateKey* 构造。任一维度超限即 429。
func RateLimitMulti(rl *RateLimiter, keyFn func(*gin.Context) []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, key := range keyFn(c) {
			if key == "" {
				continue
			}
			if !rl.allow(key) {
				ObserveRateLimit()
				AbortTooMany(c, "rate_limit_multi", "rate limit exceeded", 1, nil)
				return
			}
		}
		c.Next()
	}
}

// Stop 停止清理 goroutine。
func (rl *RateLimiter) Stop() {
	rl.cleanupT.Stop()
}

// MemorySharedCounter 进程内共享计数（单测 / 单 Pod 兜底，接口兼容 Redis）。
// 所有 entry 均带 TTL，后台清理防止无界增长。
type MemorySharedCounter struct {
	mu      sync.Mutex
	counts  map[string]*memCount
	buckets map[string]*memTokenBucket
	nonces  map[string]time.Time
	stopCh  chan struct{}
	once    sync.Once
}

type memCount struct {
	n       int64
	expires time.Time
}

type memTokenBucket struct {
	tokens  float64
	last    time.Time
	expires time.Time
}

// NewMemorySharedCounter 构造并启动 TTL 清理。
func NewMemorySharedCounter() *MemorySharedCounter {
	m := &MemorySharedCounter{
		counts:  make(map[string]*memCount),
		buckets: make(map[string]*memTokenBucket),
		nonces:  make(map[string]time.Time),
		stopCh:  make(chan struct{}),
	}
	go m.cleanupLoop()
	return m
}

// Stop 停止后台清理（测试用）。
func (m *MemorySharedCounter) Stop() {
	m.once.Do(func() { close(m.stopCh) })
}

func (m *MemorySharedCounter) cleanupLoop() {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-m.stopCh:
			return
		case <-t.C:
			m.purgeExpired()
		}
	}
}

func (m *MemorySharedCounter) purgeExpired() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for k, c := range m.counts {
		if now.After(c.expires) {
			delete(m.counts, k)
		}
	}
	for k, b := range m.buckets {
		if now.After(b.expires) {
			delete(m.buckets, k)
		}
	}
	for k, exp := range m.nonces {
		if now.After(exp) {
			delete(m.nonces, k)
		}
	}
}

// Incr 实现 SharedCounter。
func (m *MemorySharedCounter) Incr(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	c, ok := m.counts[key]
	if !ok || now.After(c.expires) {
		c = &memCount{n: 0, expires: now.Add(ttl)}
		m.counts[key] = c
	}
	c.n++
	return c.n, nil
}

// AllowToken 真实令牌桶（rate 保留小数，不截断为固定窗口）。
func (m *MemorySharedCounter) AllowToken(ctx context.Context, key string, rate float64, burst int) (bool, error) {
	_ = ctx
	if burst < 1 {
		burst = 1
	}
	if rate < 0 {
		rate = 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	// TTL：按 burst/rate 估算桶空闲寿命，至少 2 分钟，最多 1 小时。
	ttl := 2 * time.Minute
	if rate > 0 {
		idle := time.Duration(float64(burst)/rate+1) * time.Second
		if idle > ttl {
			ttl = idle
		}
		if ttl > time.Hour {
			ttl = time.Hour
		}
	}
	b, ok := m.buckets[key]
	if !ok || now.After(b.expires) {
		m.buckets[key] = &memTokenBucket{
			tokens:  float64(burst) - 1,
			last:    now,
			expires: now.Add(ttl),
		}
		return true, nil
	}
	elapsed := now.Sub(b.last).Seconds()
	b.tokens += elapsed * rate
	if b.tokens > float64(burst) {
		b.tokens = float64(burst)
	}
	b.last = now
	b.expires = now.Add(ttl)
	if b.tokens < 1 {
		return false, nil
	}
	b.tokens--
	return true, nil
}

// SetNX 实现 SharedCounter：首次设置成功返回 true。
func (m *MemorySharedCounter) SetNX(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	_ = ctx
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	if exp, ok := m.nonces[key]; ok && now.Before(exp) {
		return false, nil
	}
	m.nonces[key] = now.Add(ttl)
	return true, nil
}

// LenForTest 返回当前内存条目数（测试 TTL 回落用）。
func (m *MemorySharedCounter) LenForTest() (counts, buckets, nonces int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.counts), len(m.buckets), len(m.nonces)
}
