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

	for i := 0; i < 3; i++ {
		ok, _, _ := dc.allow("device-1", "detect")
		assert.True(t, ok, "detect %d should pass", i)
	}
	ok, _, _ := dc.allow("device-1", "detect")
	assert.False(t, ok, "4th detect should be blocked")

	ok, _, _ = dc.allow("device-1", "analyze")
	assert.True(t, ok)
	ok, _, _ = dc.allow("device-1", "analyze")
	assert.True(t, ok)
	ok, _, _ = dc.allow("device-1", "analyze")
	assert.False(t, ok)

	ok, _, _ = dc.allow("device-1", "value")
	assert.True(t, ok)
	ok, _, _ = dc.allow("device-1", "value")
	assert.False(t, ok)
}

func TestDailyCallCounter_CrossDevice(t *testing.T) {
	dc := NewDailyCallCounter(DailyLimitConfig{DetectLimit: 1})

	ok, _, _ := dc.allow("device-a", "detect")
	assert.True(t, ok)
	ok, _, _ = dc.allow("device-a", "detect")
	assert.False(t, ok)
	ok, _, _ = dc.allow("device-b", "detect")
	assert.True(t, ok)
}

func TestDailyCallCounter_CrossDay(t *testing.T) {
	dc := NewDailyCallCounter(DailyLimitConfig{DetectLimit: 1})

	ok, _, _ := dc.allow("device-1", "detect")
	assert.True(t, ok)
	ok, _, _ = dc.allow("device-1", "detect")
	assert.False(t, ok)

	dc.mu.Lock()
	d := dc.counters["device-1"]
	d.detect.date = time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	dc.mu.Unlock()

	ok, _, _ = dc.allow("device-1", "detect")
	assert.True(t, ok)
}

func TestDailyCallCounter_UnknownType(t *testing.T) {
	dc := NewDailyCallCounter(DefaultDailyLimits)
	ok, _, _ := dc.allow("device-1", "unknown-type")
	assert.True(t, ok)
}
