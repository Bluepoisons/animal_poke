package services

import (
	"fmt"
	"testing"

	"animalpoke/backend/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func midQualityInput(species string) ValueInput {
	return ValueInput{
		Species:             species,
		Breed:               "British Shorthair",
		Color:               "blue-gray",
		BodyType:            "sturdy",
		SubjectCompleteness: 7,
		Clarity:             7,
		Lighting:            6,
		Composition:         7,
		Pose:                6,
		Angle:               7,
	}
}

func TestComputeDeterministicValue_SameSeedIdentical(t *testing.T) {
	in := midQualityInput("cat")
	secret := "test-stats-secret-32-chars-long!!"
	a := ComputeDeterministicValue(in, "inf-parent-001", secret, StatsConfigVersion)
	b := ComputeDeterministicValue(in, "inf-parent-001", secret, StatsConfigVersion)
	require.NotNil(t, a)
	require.NotNil(t, b)
	assert.Equal(t, a.Rarity, b.Rarity)
	assert.Equal(t, a.HP, b.HP)
	assert.Equal(t, a.ATK, b.ATK)
	assert.Equal(t, a.DEF, b.DEF)
	assert.Equal(t, a.SPD, b.SPD)
	assert.Equal(t, a.Class, b.Class)
	assert.Equal(t, a.Element, b.Element)
	assert.Equal(t, StatsConfigVersion, a.ConfigVersion)
	require.NotNil(t, a.Factors)
	assert.Equal(t, "inf-parent-001", a.Factors.SeedID)
	assert.NotEmpty(t, a.Factors.QualityBand)
}

func TestComputeDeterministicValue_DifferentSeedsVary(t *testing.T) {
	in := midQualityInput("cat")
	secret := "test-stats-secret-32-chars-long!!"
	seen := map[string]bool{}
	for i := range 40 {
		r := ComputeDeterministicValue(in, fmt.Sprintf("seed-%d", i), secret, StatsConfigVersion)
		key := fmt.Sprintf("%d|%d|%d|%d|%d|%s|%s", r.Rarity, r.HP, r.ATK, r.DEF, r.SPD, r.Class, r.Element)
		seen[key] = true
	}
	assert.Greater(t, len(seen), 5, "different seed ids should produce varied stats")
}

func TestComputeDeterministicValue_RangesAndFactors(t *testing.T) {
	in := midQualityInput("dog")
	r := ComputeDeterministicValue(in, "seed-range", "secret", StatsConfigVersion)
	assert.GreaterOrEqual(t, r.Rarity, 1)
	assert.LessOrEqual(t, r.Rarity, 5)
	assert.GreaterOrEqual(t, r.HP, 10)
	assert.LessOrEqual(t, r.HP, 100)
	assert.GreaterOrEqual(t, r.ATK, 5)
	assert.LessOrEqual(t, r.ATK, 50)
	assert.NoError(t, r.Validate()) // narrative empty ok? Validate checks narrative length only
	// Validate requires class/element; narrative empty is fine
	require.NotNil(t, r.Factors)
	assert.GreaterOrEqual(t, r.Factors.PhotoQuality, 0.0)
	assert.LessOrEqual(t, r.Factors.PhotoQuality, 1.0)
	assert.GreaterOrEqual(t, r.Factors.Completeness, 0.0)
	assert.LessOrEqual(t, r.Factors.Completeness, 1.0)
	assert.NotZero(t, r.Factors.SpeciesWeight)
	assert.NotZero(t, r.Factors.BreedWeight)
}

func TestComputeDeterministicValue_Distribution10000(t *testing.T) {
	secret := "dist-secret"
	in := midQualityInput("cat")
	counts := map[int]int{}
	var totalPower int
	const N = 10000
	for i := range N {
		r := ComputeDeterministicValue(in, fmt.Sprintf("dist-%d", i), secret, StatsConfigVersion)
		counts[r.Rarity]++
		totalPower += r.HP + r.ATK + r.DEF + r.SPD
	}
	// Target bands (docs 5.1) with loose tolerance for factor shift
	// Common ~60%, Uncommon ~25%, Rare ~10%, Epic ~4%, Legendary ~1%
	pct := func(r int) float64 { return 100.0 * float64(counts[r]) / float64(N) }
	t.Logf("rarity distribution: 1=%.1f%% 2=%.1f%% 3=%.1f%% 4=%.1f%% 5=%.1f%%",
		pct(1), pct(2), pct(3), pct(4), pct(5))

	assert.Greater(t, counts[1], counts[5], "common > legendary")
	assert.Greater(t, counts[1], counts[4], "common > epic")
	// bands (percentage points)
	assert.InDelta(t, 60.0, pct(1), 18.0, "common ~60%%")
	assert.InDelta(t, 25.0, pct(2), 15.0, "uncommon ~25%%")
	assert.InDelta(t, 10.0, pct(3), 10.0, "rare ~10%%")
	assert.InDelta(t, 4.0, pct(4), 6.0, "epic ~4%%")
	assert.InDelta(t, 1.0, pct(5), 4.0, "legendary ~1%%")
	// battle strength mid band
	avgPower := float64(totalPower) / float64(N)
	t.Logf("avg total power HP+ATK+DEF+SPD = %.1f", avgPower)
	assert.Greater(t, avgPower, 60.0)
	assert.Less(t, avgPower, 220.0)
}

func TestGenerateValue_DeterministicWithSeed(t *testing.T) {
	cfg := &config.ThirdPartyConfig{}
	svc := NewLLMService(cfg).WithStatsSecret("unit-test-secret")
	in := midQualityInput("goose")
	in.SeedID = "same-inference-id"
	a, err := svc.GenerateValue(in)
	require.NoError(t, err)
	b, err := svc.GenerateValue(in)
	require.NoError(t, err)
	assert.Equal(t, a.Rarity, b.Rarity)
	assert.Equal(t, a.HP, b.HP)
	assert.Equal(t, a.ATK, b.ATK)
	assert.Equal(t, a.Class, b.Class)
	assert.Equal(t, "mock", a.Source)
	assert.NotNil(t, a.Factors)
	assert.Equal(t, StatsConfigVersion, a.ConfigVersion)
}

func TestGenerateValue_DifferentSeedsVary(t *testing.T) {
	cfg := &config.ThirdPartyConfig{}
	svc := NewLLMService(cfg).WithStatsSecret("unit-test-secret")
	in1 := midQualityInput("cat")
	in1.SeedID = "id-A"
	in2 := midQualityInput("cat")
	in2.SeedID = "id-B"
	a, err := svc.GenerateValue(in1)
	require.NoError(t, err)
	b, err := svc.GenerateValue(in2)
	require.NoError(t, err)
	// Extremely unlikely all equal under different seeds
	same := a.Rarity == b.Rarity && a.HP == b.HP && a.ATK == b.ATK && a.DEF == b.DEF && a.SPD == b.SPD
	assert.False(t, same, "different inference ids should usually vary stats")
}
