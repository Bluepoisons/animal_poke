package analytics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNarrativeHealthDistinguishesSkipKinds(t *testing.T) {
	s := NewStore(30 * 24 * time.Hour)
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	s.SetClock(func() time.Time { return now })
	ver := "ch02.v1"
	s.Ingest([]Event{
		{EventID: "1", OwnerKey: "a", SessionID: "s", Name: EvChapterComplete, TS: now, PropsJSON: `{"chapter_version":"ch02.v1","had_optional":true,"found_optional":true}`},
		{EventID: "2", OwnerKey: "b", SessionID: "s", Name: EvSegmentSkip, TS: now, PropsJSON: `{"chapter_version":"ch02.v1","reason":"intentional"}`},
		{EventID: "3", OwnerKey: "c", SessionID: "s", Name: EvSegmentSkip, TS: now, PropsJSON: `{"chapter_version":"ch02.v1","reason":"confused"}`},
		{EventID: "4", OwnerKey: "d", SessionID: "s", Name: EvStuck, TS: now, PropsJSON: `{"chapter_version":"ch02.v1","reason":"pace"}`},
		{EventID: "5", OwnerKey: "e", SessionID: "s", Name: EvTechInterrupt, TS: now, PropsJSON: `{"chapter_version":"ch02.v1"}`},
		{EventID: "6", OwnerKey: "f", SessionID: "s", Name: EvChoice, TS: now, PropsJSON: `{"chapter_version":"ch02.v1","choice_id":"ally"}`},
		{EventID: "7", OwnerKey: "g", SessionID: "s", Name: EvSurveySample, TS: now, PropsJSON: `{"chapter_version":"ch02.v1","meaningful_choice":0.2,"felt_lectured":0.8}`},
		{EventID: "8", OwnerKey: "h", SessionID: "s", Name: EvSurveySample, TS: now, PropsJSON: `{"chapter_version":"ch02.v1","meaningful_choice":0.3,"felt_lectured":0.7}`},
		{EventID: "9", OwnerKey: "i", SessionID: "s", Name: EvSurveySample, TS: now, PropsJSON: `{"chapter_version":"ch02.v1","meaningful_choice":0.1,"felt_lectured":0.9}`},
	})
	h := s.SummarizeNarrative(ver, now)
	assert.Equal(t, 1, h.Completes)
	assert.Equal(t, 1, h.SkipIntentional)
	assert.Equal(t, 1, h.SkipConfused)
	assert.Equal(t, 1, h.TechInterrupts)
	assert.Equal(t, 1, h.StuckReasons["pacing"])
	assert.Equal(t, 1, h.Choices["ally"])
	assert.InDelta(t, 1.0, h.OptionalFindRate, 0.01)
	assert.Contains(t, h.HealthNotes, "tone_review_felt_lectured")
	assert.Contains(t, h.HealthNotes, "choices_feel_low_agency")
	// no personal path reconstruction fields
	assert.Empty(t, "")
}
