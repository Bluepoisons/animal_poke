package taxonomy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalize_Aliases(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"cat", SpeciesCat},
		{"Cat", SpeciesCat},
		{"kitten", SpeciesCat},
		{"英短猫", SpeciesCat},
		{"dog", SpeciesDog},
		{"小狗", SpeciesDog},
		{"golden retriever dog", SpeciesDog},
		{"goose", SpeciesGoose},
		{"大鹅", SpeciesGoose},
		{"gosling", SpeciesGoose},
		// 禁止默认鹅
		{"duck", SpeciesUnsupported},
		{"swan", SpeciesUnsupported},
		{"bird", SpeciesUnsupported},
		{"鸟", SpeciesUnsupported},
		{"鸭子", SpeciesUnsupported},
		{"天鹅", SpeciesUnsupported},
		{"human", SpeciesUnsupported},
		{"person", SpeciesUnsupported},
		{"人", SpeciesUnsupported},
		{"toy", SpeciesUnsupported},
		{"玩偶", SpeciesUnsupported},
		{"screen", SpeciesUnsupported},
		{"", SpeciesUnknown},
		{"   ", SpeciesUnknown},
		{"horse", SpeciesUnknown},
		{"cow", SpeciesUnknown},
		{"mongoose", SpeciesUnsupported},
	}
	for _, tc := range cases {
		got, _ := Normalize(tc.raw)
		assert.Equal(t, tc.want, got, "raw=%q", tc.raw)
	}
}

func TestCapturable(t *testing.T) {
	assert.True(t, Capturable(SpeciesCat))
	assert.True(t, Capturable(SpeciesDog))
	assert.True(t, Capturable(SpeciesGoose))
	assert.False(t, Capturable(SpeciesUnknown))
	assert.False(t, Capturable(SpeciesUnsupported))
	assert.False(t, Capturable("bird"))
}

func TestPartition_StableSortAndFilter(t *testing.T) {
	items := []DetectLike{
		{Species: "bird", Confidence: 0.99, Index: 0},
		{Species: "dog", Confidence: 0.8, Index: 1},
		{Species: "cat", Confidence: 0.9, Index: 2},
		{Species: "duck", Confidence: 0.95, Index: 3},
		{Species: "goose", Confidence: 0.8, Index: 4},
	}
	cap, audit := Partition(items)
	assert.Len(t, cap, 3)
	assert.Equal(t, SpeciesCat, cap[0].Species)
	assert.Equal(t, 0.9, cap[0].Confidence)
	// dog and goose same conf; species order then index
	assert.True(t, len(audit) >= 2)
	for _, a := range audit {
		assert.False(t, Capturable(a.Species))
	}
}
