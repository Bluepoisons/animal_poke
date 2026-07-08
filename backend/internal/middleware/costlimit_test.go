package middleware

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDailyCallCounter_Allow(t *testing.T) {
	limits := DailyLimitConfig{
		DetectLimit:  3,
		AnalyzeLimit: 2,
		ValueLimit:   1,
	}
	dc := NewDailyCallCounter(limits)

	// detect: 前 3 次应放行
	for i := 0; i < 3; i++ {
		assert.True(t, dc.allow("device-1", "detect"), "detect %d should pass", i)
	}
	assert.False(t, dc.allow("device-1", "detect"), "4th detect should be blocked")

	// analyze: 前 2 次放行
	assert.True(t, dc.allow("device-1", "analyze"))
	assert.True(t, dc.allow("device-1", "analyze"))
	assert.False(t, dc.allow("device-1", "analyze"))

	// value: 仅 1 次
	assert.True(t, dc.allow("device-1", "value"))
	assert.False(t, dc.allow("device-1", "value"))
}

func TestDailyCallCounter_CrossDevice(t *testing.T) {
	dc := NewDailyCallCounter(DailyLimitConfig{DetectLimit: 1})

	assert.True(t, dc.allow("device-a", "detect"))
	assert.False(t, dc.allow("device-a", "detect"))
	assert.True(t, dc.allow("device-b", "detect")) // 不同设备不受影响
}

func TestDailyCallCounter_CrossDay(t *testing.T) {
	dc := NewDailyCallCounter(DailyLimitConfig{DetectLimit: 1})

	assert.True(t, dc.allow("device-1", "detect"))
	assert.False(t, dc.allow("device-1", "detect"))

	// 模拟跨天: 手动将日期设为昨天
	dc.mu.Lock()
	d := dc.counters["device-1"]
	d.detect.date = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	dc.mu.Unlock()

	// 现在应重新放行
	assert.True(t, dc.allow("device-1", "detect"))
}

func TestDailyCallCounter_UnknownType(t *testing.T) {
	dc := NewDailyCallCounter(DefaultDailyLimits)
	assert.True(t, dc.allow("device-1", "unknown-type"))
}
