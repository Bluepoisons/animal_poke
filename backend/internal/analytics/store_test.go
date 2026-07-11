package analytics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIngestDedupeLateAndDelete(t *testing.T) {
	s := NewStore(48 * time.Hour)
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	s.SetClock(func() time.Time { return now })

	a, d, dup := s.Ingest([]Event{
		{EventID: "e1", OwnerKey: "u1", SessionID: "s1", Name: "auth", TS: now},
		{EventID: "e1", OwnerKey: "u1", SessionID: "s1", Name: "auth", TS: now},
		{EventID: "e2", OwnerKey: "u1", SessionID: "s1", Name: "nope", TS: now},
		{EventID: "e3", OwnerKey: "u1", SessionID: "s1", Name: "scan", TS: now.Add(-72 * time.Hour)},
	})
	assert.Equal(t, 1, a)
	assert.Equal(t, 2, d) // unknown + late
	assert.Equal(t, 1, dup)

	n := s.DeleteOwner("u1")
	assert.Equal(t, 1, n)
	sum := s.Summarize(now, 24*time.Hour)
	assert.Equal(t, 0, sum.DAU)
	assert.Equal(t, 0, sum.EventCount)
}

func TestGoldFunnelAndRetention(t *testing.T) {
	s := NewStore(30 * 24 * time.Hour)
	asOf := time.Date(2026, 7, 11, 18, 0, 0, 0, time.UTC)
	s.SetClock(func() time.Time { return asOf })

	// Fixed dataset (gold)
	day := func(offset int, hour int) time.Time {
		return time.Date(2026, 7, 11-offset, hour, 0, 0, 0, time.UTC)
	}
	events := []Event{
		// u1 first seen yesterday, active today → D1 retained
		{EventID: "a", OwnerKey: "u1", SessionID: "s", Name: "app_open", TS: day(1, 10)},
		{EventID: "b", OwnerKey: "u1", SessionID: "s", Name: "scan", TS: day(0, 9)},
		// u2 first seen yesterday, not today → D1 not retained
		{EventID: "c", OwnerKey: "u2", SessionID: "s", Name: "app_open", TS: day(1, 11)},
		// funnel today
		{EventID: "d", OwnerKey: "u3", SessionID: "s", Name: "detect_result", TS: day(0, 12), PropsJSON: `{"outcome":"success"}`, Region: "CN", AppVersion: "1.2.0", ExperimentID: "expA", ExperimentVariant: "B"},
		{EventID: "e", OwnerKey: "u3", SessionID: "s", Name: "capture_attempt", TS: day(0, 12), PropsJSON: `{"outcome":"success"}`},
		{EventID: "f", OwnerKey: "u3", SessionID: "s", Name: "collection_complete", TS: day(0, 13)},
		{EventID: "g", OwnerKey: "u4", SessionID: "s", Name: "sync_fail", TS: day(0, 14)},
		// D7 cohort
		{EventID: "h", OwnerKey: "u5", SessionID: "s", Name: "app_open", TS: day(7, 8)},
		{EventID: "i", OwnerKey: "u5", SessionID: "s", Name: "auth", TS: day(0, 8)},
		{EventID: "j", OwnerKey: "u6", SessionID: "s", Name: "app_open", TS: day(7, 9)},
	}
	a, d, dup := s.Ingest(events)
	require.Equal(t, 0, d)
	require.Equal(t, 0, dup)
	require.Equal(t, len(events), a)

	sum := s.Summarize(asOf, 24*time.Hour)
	// DAU: u1,u3,u4,u5 (u2 only yesterday)
	assert.Equal(t, 4, sum.DAU)
	assert.Equal(t, 2, sum.Captures) // capture_attempt + collection_complete
	assert.Equal(t, 2, sum.CaptureSuccess)
	assert.Equal(t, 1, sum.DetectOK)
	assert.Equal(t, 1, sum.SyncFailures)
	assert.Equal(t, 1, sum.Funnel["scan"])
	assert.InDelta(t, 0.5, sum.D1Retention, 0.001) // u1 yes, u2 no
	assert.InDelta(t, 0.5, sum.D7Retention, 0.001) // u5 yes, u6 no
	assert.Equal(t, 1, sum.ByRegion["CN"])
	assert.Equal(t, 1, sum.ByVersion["1.2.0"])
	assert.Equal(t, 1, sum.ByExperiment["expA:B"])
	assert.Equal(t, "product-analytics", sum.DictionaryOwner)
	assert.LessOrEqual(t, sum.LatencyBoundSec, 60)
}

func TestDictionaryNonEmpty(t *testing.T) {
	assert.NotEmpty(t, Dictionary())
}
