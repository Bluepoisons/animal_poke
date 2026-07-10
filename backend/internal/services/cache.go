// Package services 有界 TTL 缓存：容量上限 LRU + singleflight 风格防击穿。
package services

import (
	"container/list"
	"sync"
	"time"

	"animalpoke/backend/internal/middleware"
)

// cacheEntry 缓存条目。
type cacheEntry[T any] struct {
	key       string
	value     T
	expiresAt time.Time
	element   *list.Element
}

// TTLCache 泛型 TTL + 容量受限 LRU 缓存。
type TTLCache[T any] struct {
	mu       sync.Mutex
	entries  map[string]*cacheEntry[T]
	lru      *list.List
	maxSize  int
	stopCh   chan struct{}
	interval time.Duration
	// singleflight
	inflight map[string]*call
}

type call struct {
	wg  sync.WaitGroup
	val interface{}
	err error
}

// NewTTLCache 创建缓存并启动后台过期清理（默认 max 4096）。
func NewTTLCache[T any](cleanupInterval time.Duration) *TTLCache[T] {
	return NewBoundedTTLCache[T](cleanupInterval, 4096)
}

// NewBoundedTTLCache 带容量上限的缓存。
func NewBoundedTTLCache[T any](cleanupInterval time.Duration, maxSize int) *TTLCache[T] {
	if maxSize <= 0 {
		maxSize = 4096
	}
	c := &TTLCache[T]{
		entries:  make(map[string]*cacheEntry[T]),
		lru:      list.New(),
		maxSize:  maxSize,
		stopCh:   make(chan struct{}),
		interval: cleanupInterval,
		inflight: make(map[string]*call),
	}
	go c.cleanupLoop()
	return c
}

// Get 获取缓存值, ok 为 false 表示不存在或已过期。
func (c *TTLCache[T]) Get(key string) (T, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok || time.Now().After(entry.expiresAt) {
		if ok {
			c.removeLocked(entry)
		}
		var zero T
		middleware.ObserveCache(false)
		return zero, false
	}
	c.lru.MoveToFront(entry.element)
	middleware.ObserveCache(true)
	return entry.value, true
}

// Set 写入缓存, ttl 为过期时长（可加 jitter 由调用方处理）。
func (c *TTLCache[T]) Set(key string, value T, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if entry, ok := c.entries[key]; ok {
		entry.value = value
		entry.expiresAt = time.Now().Add(ttl)
		c.lru.MoveToFront(entry.element)
		return
	}
	for c.lru.Len() >= c.maxSize {
		c.evictOldestLocked()
	}
	el := c.lru.PushFront(key)
	c.entries[key] = &cacheEntry[T]{
		key:       key,
		value:     value,
		expiresAt: time.Now().Add(ttl),
		element:   el,
	}
}

// Delete 删除缓存条目。
func (c *TTLCache[T]) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if entry, ok := c.entries[key]; ok {
		c.removeLocked(entry)
	}
}

// Len 返回当前缓存条目数。
func (c *TTLCache[T]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// MaxSize 返回容量上限。
func (c *TTLCache[T]) MaxSize() int {
	return c.maxSize
}

// Do 同 key 并发只执行一次 loader（singleflight）。
func (c *TTLCache[T]) Do(key string, loader func() (T, error)) (T, error) {
	if v, ok := c.Get(key); ok {
		return v, nil
	}
	c.mu.Lock()
	if cl, ok := c.inflight[key]; ok {
		c.mu.Unlock()
		cl.wg.Wait()
		if cl.err != nil {
			var zero T
			return zero, cl.err
		}
		return cl.val.(T), nil
	}
	cl := &call{}
	cl.wg.Add(1)
	c.inflight[key] = cl
	c.mu.Unlock()

	val, err := loader()
	cl.val = val
	cl.err = err
	cl.wg.Done()

	c.mu.Lock()
	delete(c.inflight, key)
	c.mu.Unlock()

	if err == nil {
		// 调用方应自行 Set；这里不强制 TTL
	}
	return val, err
}

func (c *TTLCache[T]) removeLocked(entry *cacheEntry[T]) {
	delete(c.entries, entry.key)
	if entry.element != nil {
		c.lru.Remove(entry.element)
	}
}

func (c *TTLCache[T]) evictOldestLocked() {
	el := c.lru.Back()
	if el == nil {
		return
	}
	key := el.Value.(string)
	if entry, ok := c.entries[key]; ok {
		c.removeLocked(entry)
	} else {
		c.lru.Remove(el)
	}
}

func (c *TTLCache[T]) cleanupLoop() {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			now := time.Now()
			for _, entry := range c.entries {
				if now.After(entry.expiresAt) {
					c.removeLocked(entry)
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
	select {
	case <-c.stopCh:
	default:
		close(c.stopCh)
	}
}
