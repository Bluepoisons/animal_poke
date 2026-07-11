package services

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func goodMetrics() PhotoMetrics {
	return PhotoMetrics{
		StabilityRMS:        0.06,
		SubjectFillRatio:    0.28,
		SubjectCenterOffset: 0.08,
		LightingScore:       0.72,
		OcclusionRatio:      0.1,
		SubjectCompleteness: 0.85,
		EstimatedDistanceM:  6,
		SensorSamples:       12,
		DeviceModel:         "test-phone",
	}
}

func TestScorePhotoQuality_Deterministic(t *testing.T) {
	secret := "test-photo-secret-32-chars-long!!"
	cal := DefaultPhotoCalibration()
	cal.Calibrated = true
	cal.SampleCount = 10
	m := goodMetrics()
	a, err := ScorePhotoQuality(m, cal, secret)
	require.NoError(t, err)
	b, err := ScorePhotoQuality(m, cal, secret)
	require.NoError(t, err)
	assert.Equal(t, a.Overall, b.Overall)
	assert.Equal(t, a.Band, b.Band)
	assert.Equal(t, a.Dimensions, b.Dimensions)
	assert.Equal(t, a.MetricsDigest, b.MetricsDigest)
	assert.Equal(t, a.Signature, b.Signature)
	assert.True(t, VerifyPhotoScoreSignature(a, secret))
	assert.Equal(t, PhotoQualityConfigVersion, a.ConfigVersion)
}

func TestScorePhotoQuality_SignatureFailsOnTamper(t *testing.T) {
	secret := "test-photo-secret-32-chars-long!!"
	r, err := ScorePhotoQuality(goodMetrics(), DefaultPhotoCalibration(), secret)
	require.NoError(t, err)
	r.Overall = 0.99
	assert.False(t, VerifyPhotoScoreSignature(r, secret))
}

func TestScorePhotoQuality_BadSensorsRejected(t *testing.T) {
	cases := []PhotoMetrics{
		{StabilityRMS: math.NaN(), SubjectFillRatio: 0.2, SubjectCenterOffset: 0.1, LightingScore: 0.5, OcclusionRatio: 0.1, SubjectCompleteness: 0.5, SensorSamples: 5},
		{StabilityRMS: 0.1, SubjectFillRatio: 1.5, SubjectCenterOffset: 0.1, LightingScore: 0.5, OcclusionRatio: 0.1, SubjectCompleteness: 0.5, SensorSamples: 5},
		{StabilityRMS: 0.1, SubjectFillRatio: 0.2, SubjectCenterOffset: 0.1, LightingScore: 0.5, OcclusionRatio: 0.1, SubjectCompleteness: 0.5, EstimatedDistanceM: math.Inf(1), SensorSamples: 5},
		{StabilityRMS: 0.1, SubjectFillRatio: 0.2, SubjectCenterOffset: 0.1, LightingScore: -1, OcclusionRatio: 0.1, SubjectCompleteness: 0.5, SensorSamples: 5},
		{StabilityRMS: 0.1, SubjectFillRatio: 0.2, SubjectCenterOffset: 0.1, LightingScore: 0.5, OcclusionRatio: 0.1, SubjectCompleteness: 0.5, SensorSamples: -3},
	}
	for i, m := range cases {
		_, err := ScorePhotoQuality(m, DefaultPhotoCalibration(), "s")
		assert.Error(t, err, "case %d should reject bad sensors", i)
		assert.Contains(t, err.Error(), "bad_sensor")
	}
}

func TestScorePhotoQuality_ForbidGetCloserForRarity(t *testing.T) {
	cal := DefaultPhotoCalibration()
	cal.Calibrated = true
	// Comfortable distance
	safe := goodMetrics()
	safe.SubjectFillRatio = 0.30
	safe.EstimatedDistanceM = 7
	// Crowding / chase
	close := goodMetrics()
	close.SubjectFillRatio = 0.90
	close.EstimatedDistanceM = 0.8

	sSafe, err := ScorePhotoQuality(safe, cal, "secret")
	require.NoError(t, err)
	sClose, err := ScorePhotoQuality(close, cal, "secret")
	require.NoError(t, err)

	assert.True(t, sSafe.RarityEligible)
	assert.False(t, sClose.RarityEligible)
	assert.True(t, sClose.ChasePenalty)
	assert.Contains(t, sClose.WelfareFlags, "too_close_do_not_chase")
	// Safe distance dimension must drop when crowding
	assert.Greater(t, sSafe.Dimensions.SafeDistance, sClose.Dimensions.SafeDistance)
	// Rarity factor for close must not exceed mid-fair cap
	assert.LessOrEqual(t, PhotoQualityForRarity(sClose), 0.45)
	assert.Greater(t, PhotoQualityForRarity(sSafe), PhotoQualityForRarity(sClose))
	// Closer must never produce higher safe_distance score
	assert.Less(t, sClose.Dimensions.SafeDistance, 0.3)
}

func TestScorePhotoQuality_DeviceCalibrationNormalizes(t *testing.T) {
	// Noisy device with higher baseline should still score well when relative motion is low
	noisyCal := PhotoCalibration{
		BaselineStabilityRMS: 0.20,
		LightingOffset:       0.05,
		SampleCount:          20,
		Calibrated:           true,
		ConfigVersion:        PhotoQualityConfigVersion,
	}
	quietCal := PhotoCalibration{
		BaselineStabilityRMS: 0.05,
		LightingOffset:       0,
		SampleCount:          20,
		Calibrated:           true,
		ConfigVersion:        PhotoQualityConfigVersion,
	}
	// Same absolute RMS relative to each baseline (~1.2x baseline)
	mNoisy := goodMetrics()
	mNoisy.StabilityRMS = 0.24
	mQuiet := goodMetrics()
	mQuiet.StabilityRMS = 0.06

	rNoisy, err := ScorePhotoQuality(mNoisy, noisyCal, "s")
	require.NoError(t, err)
	rQuiet, err := ScorePhotoQuality(mQuiet, quietCal, "s")
	require.NoError(t, err)
	// Relative stability should be similar after calibration
	assert.InDelta(t, rQuiet.Dimensions.Stability, rNoisy.Dimensions.Stability, 0.15)
}

func TestScorePhotoQuality_SparseSensorsCapped(t *testing.T) {
	cal := DefaultPhotoCalibration()
	cal.Calibrated = true
	// Claim perfect stillness with only 1 sample — should not max stability
	m := goodMetrics()
	m.StabilityRMS = 0.001
	m.SensorSamples = 1
	r, err := ScorePhotoQuality(m, cal, "s")
	require.NoError(t, err)
	assert.Less(t, r.Dimensions.Stability, 0.95, "sparse sensors must not farm perfect stability")
}

func TestBuildCalibration_FiltersBadSamples(t *testing.T) {
	cal := BuildCalibration(
		[]float64{0.08, 0.09, math.NaN(), 10, -1, 0.07},
		[]float64{0.02, math.Inf(1), -0.5},
	)
	assert.True(t, cal.Calibrated)
	assert.Equal(t, 3, cal.SampleCount)
	assert.InDelta(t, 0.08, cal.BaselineStabilityRMS, 0.02)
	assert.GreaterOrEqual(t, cal.LightingOffset, -0.3)
	assert.LessOrEqual(t, cal.LightingOffset, 0.3)
}

func TestDailyPhotoTheme_DeterministicAndHasA11y(t *testing.T) {
	day := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	a := DailyPhotoTheme(day)
	b := DailyPhotoTheme(day)
	assert.Equal(t, a, b)
	assert.Equal(t, "2026-07-11", a.Date)
	assert.NotEmpty(t, a.ThemeID)
	assert.NotEmpty(t, a.TargetDimension)
	assert.NotEmpty(t, a.AccessibilityAlternative.GoalID)
	assert.NotEmpty(t, a.AccessibilityAlternative.Mode)
	assert.Greater(t, a.TargetScore, 0.0)

	// Different day can differ (probabilistically across catalog)
	other := DailyPhotoTheme(day.Add(24 * time.Hour * 3))
	// At least structure valid
	assert.NotEmpty(t, other.ThemeID)
	assert.NotEmpty(t, other.AccessibilityAlternative.GoalID)
}

func TestThemeMet_A11yAlternative(t *testing.T) {
	theme := DailyPhotoTheme(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	// Low score without a11y
	low := goodMetrics()
	low.StabilityRMS = 2.0
	low.SubjectCompleteness = 0.1
	low.LightingScore = 0.1
	low.OcclusionRatio = 0.9
	low.SubjectFillRatio = 0.05
	score, err := ScorePhotoQuality(low, DefaultPhotoCalibration(), "s")
	require.NoError(t, err)
	// a11y path always counts
	assert.True(t, ThemeMet(theme, score, true))
}

func TestPhotoQualityForRarity_NoChaseBoost(t *testing.T) {
	cal := DefaultPhotoCalibration()
	cal.Calibrated = true
	// Excellent lighting/completeness but too close
	m := goodMetrics()
	m.SubjectFillRatio = 0.95
	m.EstimatedDistanceM = 0.5
	m.SubjectCompleteness = 1
	m.LightingScore = 1
	m.OcclusionRatio = 0
	m.StabilityRMS = 0.02
	r, err := ScorePhotoQuality(m, cal, "s")
	require.NoError(t, err)
	assert.True(t, r.ChasePenalty)
	assert.LessOrEqual(t, PhotoQualityForRarity(r), 0.45)
}

func TestScorePhotoQuality_TipsExplain(t *testing.T) {
	m := goodMetrics()
	m.StabilityRMS = 1.5
	m.LightingScore = 0.15
	r, err := ScorePhotoQuality(m, DefaultPhotoCalibration(), "s")
	require.NoError(t, err)
	assert.NotEmpty(t, r.Tips)
	assert.LessOrEqual(t, len(r.Tips), 3)
}

func TestAllThemesHaveA11y(t *testing.T) {
	// Probe many days — every theme entry must have a11y
	seen := map[string]bool{}
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range 60 {
		th := DailyPhotoTheme(start.AddDate(0, 0, i))
		seen[th.ThemeID] = true
		assert.NotEmpty(t, th.AccessibilityAlternative.GoalID, th.ThemeID)
		assert.NotEmpty(t, th.AccessibilityAlternative.Mode, th.ThemeID)
	}
	assert.GreaterOrEqual(t, len(seen), 4, "should cover multiple themes across 60 days")
}
