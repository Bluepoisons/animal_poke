// Package services 内存 TTL 缓存, 用于地级市级别的第三方 API 结果缓存。
package services

import (
	"sync"
	"time"
)

// cacheEntry 缓存条目。
type cacheEntry[T any] struct {
	value     T
	expiresAt time.Time
}

// TTLCache 泛型 TTL 缓存(内存, 非并发安全——调用方通过 Get/Set 内部加锁)。
type TTLCache[T any] struct {
	mu       sync.RWMutex
	entries  map[string]cacheEntry[T]
	stopCh   chan struct{}
	interval time.Duration
}

// NewTTLCache 创建缓存并启动后台过期清理。
func NewTTLCache[T any](cleanupInterval time.Duration) *TTLCache[T] {
	c := &TTLCache[T]{
		entries:  make(map[string]cacheEntry[T]),
		stopCh:   make(chan struct{}),
		interval: cleanupInterval,
	}
	go c.cleanupLoop()
	return c
}

// Get 获取缓存值, ok 为 false 表示不存在或已过期。
func (c *TTLCache[T]) Get(key string) (T, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[key]
	if !ok || time.Now().After(entry.expiresAt) {
		var zero T
		return zero, false
	}
	return entry.value, true
}

// Set 写入缓存, ttl 为过期时长。
func (c *TTLCache[T]) Set(key string, value T, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = cacheEntry[T]{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
}

// Delete 删除缓存条目。
func (c *TTLCache[T]) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}

// Len 返回当前缓存条目数。
func (c *TTLCache[T]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// cleanupLoop 后台定期清理过期条目。
func (c *TTLCache[T]) cleanupLoop() {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			now := time.Now()
			for k, entry := range c.entries {
				if now.After(entry.expiresAt) {
					delete(c.entries, k)
				}
			}
			c.mu.Unlock()
		case <-c.stopCh:
			return
		}
	}
}

// Stop 停止后台清理。
func (c *TTLCache[T]) Stop() {
	close(c.stopCh)
}
