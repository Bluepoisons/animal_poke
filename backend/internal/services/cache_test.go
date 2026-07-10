package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTTLCache_SetGet(t *testing.T) {
	c := NewTTLCache[string](time.Minute)
	defer c.Stop()

	c.Set("key1", "value1", time.Hour)
	v, ok := c.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, "value1", v)
}

func TestTTLCache_Expire(t *testing.T) {
	c := NewTTLCache[string](time.Minute)
	defer c.Stop()

	c.Set("key1", "value1", 10*time.Millisecond)
	time.Sleep(20 * time.Millisecond)

	_, ok := c.Get("key1")
	assert.False(t, ok, "过期条目应不可获取")
}

func TestTTLCache_Miss(t *testing.T) {
	c := NewTTLCache[string](time.Minute)
	defer c.Stop()

	_, ok := c.Get("nonexistent")
	assert.False(t, ok)
}

func TestTTLCache_Delete(t *testing.T) {
	c := NewTTLCache[string](time.Minute)
	defer c.Stop()

	c.Set("key1", "value1", time.Hour)
	c.Delete("key1")
	_, ok := c.Get("key1")
	assert.False(t, ok)
}

func TestTTLCache_Len(t *testing.T) {
	c := NewTTLCache[string](time.Minute)
	defer c.Stop()

	assert.Equal(t, 0, c.Len())
	c.Set("a", "1", time.Hour)
	c.Set("b", "2", time.Hour)
	assert.Equal(t, 2, c.Len())
}

func TestTTLCache_IntType(t *testing.T) {
	c := NewTTLCache[int](time.Minute)
	defer c.Stop()

	c.Set("count", 42, time.Hour)
	v, ok := c.Get("count")
	assert.True(t, ok)
	assert.Equal(t, 42, v)
}

func TestTTLCache_BoundedEviction(t *testing.T) {
	c := NewBoundedTTLCache[int](time.Minute, 2)
	defer c.Stop()
	c.Set("a", 1, time.Hour)
	c.Set("b", 2, time.Hour)
	c.Set("c", 3, time.Hour)
	assert.LessOrEqual(t, c.Len(), 2)
	_, ok := c.Get("a")
	assert.False(t, ok, "LRU 应淘汰最旧 a")
}

func TestTTLCache_DoSingleflight(t *testing.T) {
	c := NewTTLCache[string](time.Minute)
	defer c.Stop()
	calls := 0
	v, err := c.Do("k", func() (string, error) {
		calls++
		return "ok", nil
	})
	assert.NoError(t, err)
	assert.Equal(t, "ok", v)
	assert.Equal(t, 1, calls)
}
