// Package middleware MB3: 每日调用次数限制(成本控制)。
package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// DailyLimitConfig 每日限额配置。
type DailyLimitConfig struct {
	DetectLimit  int `json:"detect_limit"`  // 每日检测上限
	AnalyzeLimit int `json:"analyze_limit"` // 每日分析上限
	ValueLimit   int `json:"value_limit"`   // 每日生成上限
}

// DefaultDailyLimits 默认每日调用上限。
var DefaultDailyLimits = DailyLimitConfig{
	DetectLimit:  100,
	AnalyzeLimit: 50,
	ValueLimit:   20,
}

// DailyCallCounter 每日调用计数器(内存, 按设备分区)。
type DailyCallCounter struct {
	mu      sync.Mutex
	counters map[string]*deviceCounters
	limits   DailyLimitConfig
}

type deviceCounters struct {
	detect  dailyCount
	analyze dailyCount
	value   dailyCount
}

type dailyCount struct {
	count int
	date  string // "2006-01-02"
}

// NewDailyCallCounter 构造计数器。
func NewDailyCallCounter(limits DailyLimitConfig) *DailyCallCounter {
	return &DailyCallCounter{
		counters: make(map[string]*deviceCounters),
		limits:   limits,
	}
}

// allow 检查设备当天的某类型调用是否已达上限。
// 返回 true 表示允许, false 则超限。
func (dc *DailyCallCounter) allow(deviceID, callType string) bool {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	today := time.Now().Format("2006-01-02")
	d, ok := dc.counters[deviceID]
	if !ok {
		d = &deviceCounters{}
		dc.counters[deviceID] = d
	}

	var c *dailyCount
	var limit int
	switch callType {
	case "detect":
		c = &d.detect
		limit = dc.limits.DetectLimit
	case "analyze":
		c = &d.analyze
		limit = dc.limits.AnalyzeLimit
	case "value":
		c = &d.value
		limit = dc.limits.ValueLimit
	default:
		return true
	}

	// 跨天重置
	if c.date != today {
		c.count = 0
		c.date = today
	}

	if c.count >= limit {
		return false
	}
	c.count++
	return true
}

// CostLimitByType 按调用类型限流中间件。
// callType: "detect" / "analyze" / "value"
func CostLimitByType(counter *DailyCallCounter, callType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		deviceID := GetDeviceID(c)
		if deviceID == "" {
			c.Next()
			return
		}
		if !counter.allow(deviceID, callType) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "daily call limit exceeded for " + callType,
			})
			return
		}
		c.Next()
	}
}
