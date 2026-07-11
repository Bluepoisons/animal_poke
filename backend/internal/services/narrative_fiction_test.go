package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNarrativeFallback_FictionNotBiography(t *testing.T) {
	in := ValueInput{Species: "dog", Breed: "mix", Color: "brown"}
	v := &ValueResult{Rarity: 2}
	s := narrativeFallback(in, v)
	assert.Contains(t, s, "虚构")
	assert.NotContains(t, s, "主人是")
	assert.NotContains(t, s, "owned by")
	assert.NotContains(t, s, "diagnosed")
}

func TestValueResult_FictionLayerDefaults(t *testing.T) {
	in := ValueInput{
		Species: "cat", Breed: "Tabby", Color: "orange", BodyType: "lean", SeedID: "seed-fic-1",
		SubjectCompleteness: 5, Clarity: 5, Lighting: 5, Composition: 5, Pose: 5, Angle: 5,
	}
	r := ComputeDeterministicValue(in, in.SeedID, "unit-test-secret-stats-hmac-key!!", StatsConfigVersion)
	r.Narrative = narrativeFallback(in, r)
	r.Fiction = true
	r.Layer = "fictional_vignette"
	r.Disclaimer = "fictional vignette; not a real animal biography"
	assert.True(t, r.Fiction)
	assert.Equal(t, "fictional_vignette", r.Layer)
	assert.NotEmpty(t, r.Disclaimer)
}
