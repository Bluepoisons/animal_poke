// Package middleware MB3: 每日调用次数限制(成本控制)，支持共享存储。
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// DailyLimitConfig 每日限额配置。
type DailyLimitConfig struct {
	DetectLimit  int `json:"detect_limit"`
	AnalyzeLimit int `json:"analyze_limit"`
	ValueLimit   int `json:"value_limit"`
}

// DefaultDailyLimits 默认每日调用上限。
var DefaultDailyLimits = DailyLimitConfig{
	DetectLimit:  100,
	AnalyzeLimit: 50,
	ValueLimit:   20,
}

// DailyCallCounter 每日调用计数器。
type DailyCallCounter struct {
	mu       sync.Mutex
	counters map[string]*deviceCounters
	limits   DailyLimitConfig
	shared   SharedCounter
}

type deviceCounters struct {
	detect  dailyCount
	analyze dailyCount
	value   dailyCount
}

type dailyCount struct {
	count int
	date  string
}

// NewDailyCallCounter 构造计数器。
func NewDailyCallCounter(limits DailyLimitConfig) *DailyCallCounter {
	return &DailyCallCounter{
		counters: make(map[string]*deviceCounters),
		limits:   limits,
	}
}

// WithShared 注入 Redis 等共享计数。
func (dc *DailyCallCounter) WithShared(s SharedCounter) *DailyCallCounter {
	dc.shared = s
	return dc
}

func (dc *DailyCallCounter) limitOf(callType string) int {
	switch callType {
	case "detect":
		return dc.limits.DetectLimit
	case "analyze":
		return dc.limits.AnalyzeLimit
	case "value":
		return dc.limits.ValueLimit
	default:
		return 0
	}
}

// allow 检查并递增；返回 (allowed, remaining, limit)。
func (dc *DailyCallCounter) allow(deviceID, callType string) (bool, int, int) {
	limit := dc.limitOf(callType)
	if limit <= 0 {
		return true, -1, 0
	}
	today := time.Now().UTC().Format("2006-01-02")
	key := fmt.Sprintf("cost:%s:%s:%s", callType, deviceID, today)

	if dc.shared != nil {
		// 次日 0 点过期
		ttl := 26 * time.Hour
		n, err := dc.shared.Incr(context.Background(), key, ttl)
		if err == nil {
			remaining := limit - int(n)
			if remaining < 0 {
				remaining = 0
			}
			return int(n) <= limit, remaining, limit
		}
	}

	dc.mu.Lock()
	defer dc.mu.Unlock()

	d, ok := dc.counters[deviceID]
	if !ok {
		d = &deviceCounters{}
		dc.counters[deviceID] = d
	}

	var c *dailyCount
	switch callType {
	case "detect":
		c = &d.detect
	case "analyze":
		c = &d.analyze
	case "value":
		c = &d.value
	default:
		return true, -1, 0
	}

	if c.date != today {
		c.count = 0
		c.date = today
	}
	if c.count >= limit {
		return false, 0, limit
	}
	c.count++
	return true, limit - c.count, limit
}

// CostLimitByType 按调用类型限流中间件。
func CostLimitByType(counter *DailyCallCounter, callType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		deviceID := GetDeviceID(c)
		if deviceID == "" {
			c.Next()
			return
		}
		ok, remaining, limit := counter.allow(deviceID, callType)
		if limit > 0 {
			c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
			c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		}
		if !ok {
			c.Header("Retry-After", "86400")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "daily call limit exceeded for " + callType,
				"limit":       limit,
				"remaining":   0,
				"retry_after": 86400,
				"reason_code": "daily_quota",
			})
			return
		}
		c.Next()
	}
}
