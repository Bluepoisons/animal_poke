package taxonomy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		{"rabbit", SpeciesRabbit},
		{"bunny", SpeciesRabbit},
		{"野兔", SpeciesRabbit},
		// 广谱鸟类：通用鸟保持 bird，不默认成鹅
		{"duck", "duck"},
		{"swan", "bird"},
		{"bird", "bird"},
		{"鸟", "bird"},
		{"鸭子", "duck"},
		{"天鹅", "bird"},
		{"horse", "horse"},
		{"snake", "snake"},
		{"青蛙", "frog"},
		{"海马", "fish"},
		{"牛蛙", "frog"},
		{"食人鱼", "fish"},
		{"河马", SpeciesUnknown},
		{"蜗牛", SpeciesUnknown},
		{"海牛", SpeciesUnknown},
		{"木马", SpeciesUnsupported},
		{"workhorse", SpeciesUnknown},
		{"caracal", SpeciesUnknown},
		{"fish", "fish"},
		{"other_animal", "other_animal"},
		{"human", SpeciesUnsupported},
		{"person", SpeciesUnsupported},
		{"人", SpeciesUnsupported},
		{"kid", SpeciesUnsupported},
		{"小孩", SpeciesUnsupported},
		{"toy", SpeciesUnsupported},
		{"玩偶", SpeciesUnsupported},
		{"screen", SpeciesUnsupported},
		{"", SpeciesUnknown},
		{"   ", SpeciesUnknown},
		{"mongoose", SpeciesUnknown},
		{"unknown animal", SpeciesUnknown},
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
	assert.True(t, Capturable(SpeciesRabbit))
	assert.True(t, Capturable("bird"))
	assert.True(t, Capturable("other_animal"))
	assert.False(t, Capturable(SpeciesUnknown))
	assert.False(t, Capturable(SpeciesUnsupported))
}

func TestCapturableSpecies_NoHardcodedSwitch(t *testing.T) {
	got := CapturableSpecies()
	assert.Len(t, got, 36)
	assert.Contains(t, got, "bird")
	assert.Contains(t, got, "rabbit")
	assert.Contains(t, got, "other_animal")
	enc := EncyclopediaSpecies()
	assert.ElementsMatch(t, got, enc)
	assert.Equal(t, "capturable", EffectiveStatus(SpeciesRabbit))
}

func TestContentRef(t *testing.T) {
	ref := ContentRef(SpeciesCat)
	assert.Equal(t, "cat", ref.ID)
	assert.Equal(t, "1.0.0", ref.Version)
}

func TestPartition_StableSortAndFilter(t *testing.T) {
	items := []DetectLike{
		{Species: "bird", Confidence: 0.99, Index: 0},
		{Species: "dog", Confidence: 0.8, Index: 1},
		{Species: "cat", Confidence: 0.9, Index: 2},
		{Species: "duck", Confidence: 0.95, Index: 3},
		{Species: "goose", Confidence: 0.8, Index: 4},
		{Species: "rabbit", Confidence: 0.99, Index: 5},
		{Species: "human", Confidence: 1, Index: 6},
		{Species: "mongoose", Confidence: 0.7, Index: 7},
	}
	cap, audit := Partition(items)
	assert.Len(t, cap, 6)
	assert.Equal(t, "bird", cap[0].Species)
	assert.Equal(t, SpeciesRabbit, cap[1].Species)
	require.Len(t, audit, 2)
	for _, a := range audit {
		assert.False(t, Capturable(a.Species))
	}
}
