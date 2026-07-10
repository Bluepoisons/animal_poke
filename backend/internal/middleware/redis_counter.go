// Redis 共享计数：Lua 令牌桶 + INCR TTL + SET NX EX。
// 依赖 go-redis；仅在 REDIS_URL 配置时启用。
package middleware

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis 脚本：令牌桶（rate 为浮点，不截断）。
// KEYS[1]=key ARGV: rate, burst, now_ms, ttl_sec
// 返回 1=允许 0=拒绝
var luaTokenBucket = redis.NewScript(`
local rate = tonumber(ARGV[1])
local burst = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local ttl = tonumber(ARGV[4])
if burst < 1 then burst = 1 end
if rate < 0 then rate = 0 end

local data = redis.call('HMGET', KEYS[1], 'tokens', 'ts')
local tokens = tonumber(data[1])
local ts = tonumber(data[2])

if tokens == nil or ts == nil then
  tokens = burst
  ts = now
end

local elapsed = (now - ts) / 1000.0
if elapsed < 0 then elapsed = 0 end
tokens = tokens + elapsed * rate
if tokens > burst then tokens = burst end

local allowed = 0
if tokens >= 1 then
  tokens = tokens - 1
  allowed = 1
end

redis.call('HSET', KEYS[1], 'tokens', tokens, 'ts', now)
redis.call('EXPIRE', KEYS[1], ttl)
return allowed
`)

// Redis 脚本：INCR + 首次 PEXPIRE。
// KEYS[1]=key ARGV[1]=ttl_ms
var luaIncrTTL = redis.NewScript(`
local n = redis.call('INCR', KEYS[1])
if n == 1 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
return n
`)

// RedisSharedCounter 基于 go-redis 的 SharedCounter 实现。
type RedisSharedCounter struct {
	client *redis.Client
}

// NewRedisSharedCounter 从 REDIS_URL（redis://...）构造；url 非法返回 error。
func NewRedisSharedCounter(redisURL string) (*RedisSharedCounter, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse REDIS_URL: %w", err)
	}
	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return &RedisSharedCounter{client: client}, nil
}

// NewRedisSharedCounterFromClient 注入已有 client（测试用）。
func NewRedisSharedCounterFromClient(client *redis.Client) *RedisSharedCounter {
	return &RedisSharedCounter{client: client}
}

// Close 关闭底层连接。
func (r *RedisSharedCounter) Close() error {
	if r == nil || r.client == nil {
		return nil
	}
	return r.client.Close()
}

// Incr 原子自增；仅首次设置 TTL。
func (r *RedisSharedCounter) Incr(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	if ttl < time.Millisecond {
		ttl = time.Second
	}
	ms := ttl.Milliseconds()
	n, err := luaIncrTTL.Run(ctx, r.client, []string{key}, ms).Int64()
	if err != nil {
		return 0, err
	}
	return n, nil
}

// AllowToken Lua 令牌桶；TTL 按桶空闲时间估算。
func (r *RedisSharedCounter) AllowToken(ctx context.Context, key string, rate float64, burst int) (bool, error) {
	if burst < 1 {
		burst = 1
	}
	ttlSec := int64(120)
	if rate > 0 {
		idle := int64(float64(burst)/rate) + 2
		if idle > ttlSec {
			ttlSec = idle
		}
		if ttlSec > 3600 {
			ttlSec = 3600
		}
	}
	nowMs := time.Now().UnixMilli()
	res, err := luaTokenBucket.Run(ctx, r.client, []string{key}, rate, burst, nowMs, ttlSec).Int64()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}

// SetNX SET key 1 NX EX ttl。
func (r *RedisSharedCounter) SetNX(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	if ttl < time.Second {
		ttl = time.Second
	}
	return r.client.SetNX(ctx, key, "1", ttl).Result()
}
