// Package services — photography quality skill scoring (AP-098).
// Observation quality is a non-intrusive skill loop: stability, completeness,
// lighting, occlusion, composition and safe distance. Scoring is deterministic
// and server-authoritative (or HMAC-signed). "Get closer" / chase never improves
// rarity or skill score; safe distance is rewarded.
package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"animalpoke/backend/internal/config"
)

// PhotoQualityConfigVersion bumps when weights/rules change (replay safety).
const PhotoQualityConfigVersion = "photo-quality-v1"

// Daily submission cap per owner (anti-farm).
const PhotoScoreDailyCap = 40

// Min sensor samples for a stable reading.
const PhotoMinSensorSamples = 3

// Fill-ratio band for composition sweet spot (subject size in frame).
// Too large (crowding/chase) is penalized via safe_distance, never rewarded.
const (
	FillSweetMin = 0.12
	FillSweetMax = 0.55
	FillTooClose = 0.72 // welfare / chase threshold
)

// PhotoMetrics is client-reported observation metrics (no raw photo bytes).
// Server validates ranges and re-scores deterministically.
type PhotoMetrics struct {
	// StabilityRMS is motion magnitude during hold (lower is better). Unit-free ~0-2.
	StabilityRMS float64 `json:"stability_rms"`
	// SubjectFillRatio is bbox area / frame area in [0,1].
	SubjectFillRatio float64 `json:"subject_fill_ratio"`
	// SubjectCenterOffset is distance of subject center from frame center in [0,1].
	SubjectCenterOffset float64 `json:"subject_center_offset"`
	// LightingScore is estimated exposure quality in [0,1] (0.5 = neutral).
	LightingScore float64 `json:"lighting_score"`
	// OcclusionRatio is estimated subject occlusion in [0,1] (0 = clear).
	OcclusionRatio float64 `json:"occlusion_ratio"`
	// SubjectCompleteness is how complete the animal appears in [0,1].
	SubjectCompleteness float64 `json:"subject_completeness"`
	// EstimatedDistanceM optional distance estimate (meters). 0 = unknown.
	EstimatedDistanceM float64 `json:"estimated_distance_m,omitempty"`
	// SensorSamples count of gyro/accel samples used for stability.
	SensorSamples int `json:"sensor_samples"`
	// DeviceModel optional coarse device class for calibration lookup.
	DeviceModel string `json:"device_model,omitempty"`
}

// PhotoCalibration normalizes device sensor bias.
type PhotoCalibration struct {
	// BaselineStabilityRMS typical idle hold RMS for this device.
	BaselineStabilityRMS float64 `json:"baseline_stability_rms"`
	// LightingOffset additive bias applied to lighting_score before clamp.
	LightingOffset float64 `json:"lighting_offset"`
	// SampleCount how many calibration samples contributed.
	SampleCount int `json:"sample_count"`
	// Calibrated whether enough samples exist.
	Calibrated bool `json:"calibrated"`
	// ConfigVersion algorithm version used when calibration was written.
	ConfigVersion string `json:"config_version"`
}

// PhotoDimensionScores are the six skill dimensions in [0,1].
type PhotoDimensionScores struct {
	Stability           float64 `json:"stability"`
	SubjectCompleteness float64 `json:"subject_completeness"`
	Lighting            float64 `json:"lighting"`
	Occlusion           float64 `json:"occlusion"` // higher = less occlusion (clearer)
	Composition         float64 `json:"composition"`
	SafeDistance        float64 `json:"safe_distance"`
}

// PhotoScoreResult is the authoritative score payload (signed when secret present).
type PhotoScoreResult struct {
	Overall       float64              `json:"overall"` // 0-1
	Band          string               `json:"band"`    // excellent|good|fair|poor
	Dimensions    PhotoDimensionScores `json:"dimensions"`
	Tips          []string             `json:"tips"`
	WelfareFlags  []string             `json:"welfare_flags,omitempty"`
	ResearchBonus float64              `json:"research_bonus"` // 0-1 skill contribution (not rarity)
	// ChasePenalty true when player crowded subject — never boosts rarity.
	ChasePenalty  bool   `json:"chase_penalty"`
	ConfigVersion string `json:"config_version"`
	MetricsDigest string `json:"metrics_digest"`
	Signature     string `json:"signature,omitempty"`
	// RarityEligible: false when welfare/chase rules ban quality→rarity contribution.
	RarityEligible bool `json:"rarity_eligible"`
}

// PhotoDailyTheme is a day-scoped photography challenge with a11y alternative.
type PhotoDailyTheme struct {
	Date            string  `json:"date"` // YYYY-MM-DD UTC
	ThemeID         string  `json:"theme_id"`
	Title           string  `json:"title"`
	Description     string  `json:"description"`
	TargetDimension string  `json:"target_dimension"`
	TargetScore     float64 `json:"target_score"`
	// AccessibilityAlternative is a non-visual / reduced-motion goal for the same day.
	AccessibilityAlternative PhotoA11yGoal `json:"accessibility_alternative"`
	ConfigVersion            string        `json:"config_version"`
}

// PhotoA11yGoal is an alternative objective for players who cannot use camera skill cues.
type PhotoA11yGoal struct {
	GoalID      string `json:"goal_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	// Mode: timer_hold | high_contrast_frame | voice_guided | static_upload
	Mode string `json:"mode"`
}

// themeCatalog is deterministic daily themes (index by day hash).
var themeCatalog = []struct {
	ID        string
	Title     string
	Desc      string
	Dim       string
	Target    float64
	A11yID    string
	A11yTitle string
	A11yDesc  string
	A11yMode  string
}{
	{
		ID: "steady_hands", Title: "Steady Hands",
		Desc: "Hold still for a sharp observation. Stability is the skill.",
		Dim:  "stability", Target: 0.75,
		A11yID: "timer_hold_3s", A11yTitle: "3-second timer hold",
		A11yDesc: "Use the accessibility timer: keep the frame locked for 3 seconds without motion feedback.",
		A11yMode: "timer_hold",
	},
	{
		ID: "soft_light", Title: "Soft Light",
		Desc: "Find even lighting — not flash, not silhouette.",
		Dim:  "lighting", Target: 0.70,
		A11yID: "high_contrast_frame", A11yTitle: "High-contrast frame guide",
		A11yDesc: "Enable high-contrast framing rings; complete one guided frame without relying on brightness meters.",
		A11yMode: "high_contrast_frame",
	},
	{
		ID: "full_subject", Title: "Whole Subject",
		Desc: "Capture a complete subject outline without crowding.",
		Dim:  "subject_completeness", Target: 0.75,
		A11yID: "voice_outline", A11yTitle: "Voice-guided outline check",
		A11yDesc: "Use voice guidance to confirm head-to-tail coverage without visual completeness meters.",
		A11yMode: "voice_guided",
	},
	{
		ID: "clear_view", Title: "Clear View",
		Desc: "Avoid fences, branches, and partial blocks.",
		Dim:  "occlusion", Target: 0.80,
		A11yID: "static_upload_clear", A11yTitle: "Static clear-view upload",
		A11yDesc: "Upload a single still from a safe vantage; skip live occlusion feedback.",
		A11yMode: "static_upload",
	},
	{
		ID: "balanced_frame", Title: "Balanced Frame",
		Desc: "Center composition with a comfortable subject size.",
		Dim:  "composition", Target: 0.70,
		A11yID: "fixed_guide_box", A11yTitle: "Fixed guide box",
		A11yDesc: "Snap once when the subject is inside the large guide box; no fine re-framing required.",
		A11yMode: "high_contrast_frame",
	},
	{
		ID: "safe_distance", Title: "Respectful Distance",
		Desc: "Observe without crowding. Closer is never better for rarity.",
		Dim:  "safe_distance", Target: 0.80,
		A11yID: "distance_voice", A11yTitle: "Distance voice cue",
		A11yDesc: "Follow the voice cue 'good distance' without needing visual distance meters or zoom chase.",
		A11yMode: "voice_guided",
	},
}

// DefaultPhotoCalibration returns a neutral calibration for uncalibrated devices.
func DefaultPhotoCalibration() PhotoCalibration {
	return PhotoCalibration{
		BaselineStabilityRMS: 0.08,
		LightingOffset:       0,
		SampleCount:          0,
		Calibrated:           false,
		ConfigVersion:        PhotoQualityConfigVersion,
	}
}

// BuildCalibration averages idle stability samples and clamps to safe ranges.
func BuildCalibration(samples []float64, lightingOffsets []float64) PhotoCalibration {
	cal := DefaultPhotoCalibration()
	if len(samples) == 0 {
		return cal
	}
	sum := 0.0
	n := 0
	for _, s := range samples {
		if math.IsNaN(s) || math.IsInf(s, 0) || s < 0 || s > 5 {
			continue
		}
		sum += s
		n++
	}
	if n == 0 {
		return cal
	}
	cal.BaselineStabilityRMS = clamp01Range(sum/float64(n), 0.02, 0.5)
	cal.SampleCount = n
	cal.Calibrated = n >= PhotoMinSensorSamples
	if len(lightingOffsets) > 0 {
		ls := 0.0
		ln := 0
		for _, o := range lightingOffsets {
			if math.IsNaN(o) || math.IsInf(o, 0) {
				continue
			}
			ls += clamp01Range(o, -0.3, 0.3)
			ln++
		}
		if ln > 0 {
			cal.LightingOffset = ls / float64(ln)
		}
	}
	return cal
}

// ValidatePhotoMetrics rejects NaN/Inf/out-of-range sensor inputs (anti-farm / bad sensors).
func ValidatePhotoMetrics(m PhotoMetrics) error {
	checks := []struct {
		name string
		v    float64
		lo   float64
		hi   float64
	}{
		{"stability_rms", m.StabilityRMS, 0, 5},
		{"subject_fill_ratio", m.SubjectFillRatio, 0, 1},
		{"subject_center_offset", m.SubjectCenterOffset, 0, 1.5},
		{"lighting_score", m.LightingScore, 0, 1},
		{"occlusion_ratio", m.OcclusionRatio, 0, 1},
		{"subject_completeness", m.SubjectCompleteness, 0, 1},
	}
	for _, c := range checks {
		if math.IsNaN(c.v) || math.IsInf(c.v, 0) {
			return fmt.Errorf("bad_sensor: %s is not finite", c.name)
		}
		if c.v < c.lo || c.v > c.hi {
			return fmt.Errorf("bad_sensor: %s out of range", c.name)
		}
	}
	if m.EstimatedDistanceM != 0 {
		if math.IsNaN(m.EstimatedDistanceM) || math.IsInf(m.EstimatedDistanceM, 0) || m.EstimatedDistanceM < 0 || m.EstimatedDistanceM > 5000 {
			return fmt.Errorf("bad_sensor: estimated_distance_m out of range")
		}
	}
	if m.SensorSamples < 0 || m.SensorSamples > 10_000 {
		return fmt.Errorf("bad_sensor: sensor_samples out of range")
	}
	return nil
}

// ScorePhotoQuality computes deterministic six-dimension scores.
// cal may be zero-value (uses DefaultPhotoCalibration).
// secret is used for HMAC signature (STATS_HMAC_KEY or dedicated key).
func ScorePhotoQuality(m PhotoMetrics, cal PhotoCalibration, secret string) (*PhotoScoreResult, error) {
	if err := ValidatePhotoMetrics(m); err != nil {
		return nil, err
	}
	if !cal.Calibrated && cal.BaselineStabilityRMS == 0 {
		cal = DefaultPhotoCalibration()
	}
	if cal.BaselineStabilityRMS <= 0 {
		cal.BaselineStabilityRMS = DefaultPhotoCalibration().BaselineStabilityRMS
	}

	// --- stability: lower RMS better; normalize by device baseline ---
	// ratio = baseline / max(rms, epsilon); clamp
	rms := m.StabilityRMS
	if m.SensorSamples > 0 && m.SensorSamples < PhotoMinSensorSamples {
		// sparse sensors: cap stability so low-end devices aren't farmed via fake zeros
		rms = math.Max(rms, cal.BaselineStabilityRMS*1.5)
	}
	stabRatio := cal.BaselineStabilityRMS / math.Max(rms, 1e-6)
	stability := clamp01(stabRatio)
	// perfect zero RMS with no samples is suspicious → treat as uncalibrated mid
	if m.SensorSamples == 0 && rms == 0 {
		stability = 0.45
	}

	// --- subject completeness ---
	completeness := clamp01(m.SubjectCompleteness)

	// --- lighting with calibration offset ---
	lighting := clamp01(m.LightingScore + cal.LightingOffset)

	// --- occlusion (invert): higher score = clearer ---
	occlusionClear := clamp01(1.0 - m.OcclusionRatio)

	// --- composition: center + fill sweet spot ---
	centerScore := clamp01(1.0 - m.SubjectCenterOffset/0.5)
	fill := m.SubjectFillRatio
	var fillScore float64
	switch {
	case fill < FillSweetMin:
		// too small / far — soft penalty (not a chase reward)
		fillScore = clamp01(fill / FillSweetMin)
	case fill <= FillSweetMax:
		fillScore = 1.0
	case fill < FillTooClose:
		// approaching too close — taper
		fillScore = clamp01(1.0 - (fill-FillSweetMax)/(FillTooClose-FillSweetMax)*0.5)
	default:
		// crowding: composition suffers
		fillScore = 0.25
	}
	composition := clamp01(centerScore*0.55 + fillScore*0.45)

	// --- safe_distance: peaks mid-range; FORBIDS "get closer" reward ---
	safe := safeDistanceScore(fill, m.EstimatedDistanceM)

	dims := PhotoDimensionScores{
		Stability:           round3(stability),
		SubjectCompleteness: round3(completeness),
		Lighting:            round3(lighting),
		Occlusion:           round3(occlusionClear),
		Composition:         round3(composition),
		SafeDistance:        round3(safe),
	}

	// Weighted overall — safe_distance always contributes (welfare-first skill).
	overall := round3(clamp01(
		dims.Stability*0.18 +
			dims.SubjectCompleteness*0.18 +
			dims.Lighting*0.16 +
			dims.Occlusion*0.14 +
			dims.Composition*0.16 +
			dims.SafeDistance*0.18,
	))

	chase := fill >= FillTooClose || (m.EstimatedDistanceM > 0 && m.EstimatedDistanceM < 1.5)
	var flags []string
	if chase {
		flags = append(flags, "too_close_do_not_chase")
	}
	if m.OcclusionRatio >= 0.7 {
		flags = append(flags, "heavy_occlusion")
	}
	if dims.Lighting < 0.3 {
		flags = append(flags, "poor_lighting")
	}

	// Research bonus rewards skill without feeding rarity when chase detected.
	research := overall
	if chase {
		research = round3(overall * 0.5)
	}

	// Rarity eligibility: never when chase/welfare flags fire.
	// Quality may still inform research/display, but must not raise rarity via proximity.
	rarityOK := !chase && overall >= 0.25

	tips := buildPhotoTips(dims, chase)

	digest := MetricsDigest(m, cal)
	res := &PhotoScoreResult{
		Overall:        overall,
		Band:           qualityBand(overall),
		Dimensions:     dims,
		Tips:           tips,
		WelfareFlags:   flags,
		ResearchBonus:  research,
		ChasePenalty:   chase,
		ConfigVersion:  PhotoQualityConfigVersion,
		MetricsDigest:  digest,
		RarityEligible: rarityOK,
	}
	res.Signature = SignPhotoScore(res, secret)
	return res, nil
}

// safeDistanceScore rewards respectful observation distance.
// fill_ratio and optional meters both map to a mid-range peak.
// Getting closer beyond the sweet spot ALWAYS lowers this score.
func safeDistanceScore(fill, distanceM float64) float64 {
	// From fill: ideal ~0.2–0.45; too close (>0.72) → near zero
	var fromFill float64
	switch {
	case fill <= 0:
		fromFill = 0.4 // unknown subject size — neutral-low
	case fill < 0.08:
		fromFill = 0.55 // very far but safe
	case fill <= 0.45:
		// ramp up then hold
		fromFill = 0.7 + 0.3*clamp01((fill-0.08)/0.20)
	case fill < FillTooClose:
		fromFill = clamp01(1.0 - (fill-0.45)/(FillTooClose-0.45))
	default:
		fromFill = 0.05 // crowded — welfare fail
	}

	if distanceM <= 0 {
		return clamp01(fromFill)
	}
	// meters: ideal 3–12m for wildlife; <1.5m fail; >40m soft ok
	var fromDist float64
	switch {
	case distanceM < 1.5:
		fromDist = 0.05
	case distanceM < 3:
		fromDist = 0.4 + 0.4*((distanceM-1.5)/1.5)
	case distanceM <= 12:
		fromDist = 1.0
	case distanceM <= 40:
		fromDist = 0.7
	default:
		fromDist = 0.55
	}
	// combine; both must be decent — min-ish blend
	return clamp01(fromFill*0.55 + fromDist*0.45)
}

func buildPhotoTips(d PhotoDimensionScores, chase bool) []string {
	type pair struct {
		score float64
		tip   string
	}
	cands := []pair{
		{d.Stability, "Hold steadier for a few seconds before capturing"},
		{d.SubjectCompleteness, "Frame the whole animal when safe to do so"},
		{d.Lighting, "Seek even natural light; avoid harsh backlight"},
		{d.Occlusion, "Move sideways for a clearer line of sight — do not approach"},
		{d.Composition, "Center the subject with comfortable framing"},
		{d.SafeDistance, "Keep a respectful distance; closer never raises rarity"},
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].score < cands[j].score })
	out := make([]string, 0, 3)
	for _, c := range cands {
		if c.score >= 0.75 {
			continue
		}
		out = append(out, c.tip)
		if len(out) >= 3 {
			break
		}
	}
	if chase {
		// Always surface welfare tip first when chasing
		out = append([]string{"Too close — step back. Do not chase or crowd animals."}, out...)
		if len(out) > 3 {
			out = out[:3]
		}
	}
	if len(out) == 0 {
		out = []string{"Great observation — keep practicing safe, steady framing"}
	}
	return out
}

// MetricsDigest fingerprints metrics for anti-farm dedupe (no PII/photo).
func MetricsDigest(m PhotoMetrics, cal PhotoCalibration) string {
	h := sha256.New()
	fmt.Fprintf(h, "%.4f|%.4f|%.4f|%.4f|%.4f|%.4f|%.2f|%d|%.4f|%.4f|%s",
		m.StabilityRMS, m.SubjectFillRatio, m.SubjectCenterOffset,
		m.LightingScore, m.OcclusionRatio, m.SubjectCompleteness,
		m.EstimatedDistanceM, m.SensorSamples,
		cal.BaselineStabilityRMS, cal.LightingOffset,
		PhotoQualityConfigVersion,
	)
	return hex.EncodeToString(h.Sum(nil))[:24]
}

// SignPhotoScore HMAC-signs the score for client trust / offline verify.
func SignPhotoScore(r *PhotoScoreResult, secret string) string {
	if r == nil {
		return ""
	}
	if secret == "" {
		secret = config.DefaultDevStatsHMACKey
	}
	payload := fmt.Sprintf("%s|%.3f|%s|%.3f|%.3f|%.3f|%.3f|%.3f|%.3f|%v|%v|%s",
		r.ConfigVersion, r.Overall, r.Band,
		r.Dimensions.Stability, r.Dimensions.SubjectCompleteness, r.Dimensions.Lighting,
		r.Dimensions.Occlusion, r.Dimensions.Composition, r.Dimensions.SafeDistance,
		r.ChasePenalty, r.RarityEligible, r.MetricsDigest,
	)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyPhotoScoreSignature checks a previously signed result.
func VerifyPhotoScoreSignature(r *PhotoScoreResult, secret string) bool {
	if r == nil || r.Signature == "" {
		return false
	}
	expected := SignPhotoScore(r, secret)
	return hmac.Equal([]byte(expected), []byte(r.Signature))
}

// DailyPhotoTheme returns the UTC-day theme (deterministic, no RNG state).
func DailyPhotoTheme(day time.Time) PhotoDailyTheme {
	date := day.UTC().Format("2006-01-02")
	idx := themeIndexForDate(date)
	t := themeCatalog[idx]
	return PhotoDailyTheme{
		Date:            date,
		ThemeID:         t.ID,
		Title:           t.Title,
		Description:     t.Desc,
		TargetDimension: t.Dim,
		TargetScore:     t.Target,
		AccessibilityAlternative: PhotoA11yGoal{
			GoalID:      t.A11yID,
			Title:       t.A11yTitle,
			Description: t.A11yDesc,
			Mode:        t.A11yMode,
		},
		ConfigVersion: PhotoQualityConfigVersion,
	}
}

func themeIndexForDate(date string) int {
	h := sha256.Sum256([]byte("photo-theme|" + date + "|" + PhotoQualityConfigVersion))
	// use first 4 bytes as uint
	n := int(h[0])<<24 | int(h[1])<<16 | int(h[2])<<8 | int(h[3])
	if n < 0 {
		n = -n
	}
	return n % len(themeCatalog)
}

// PhotoQualityForRarity maps skill overall into the 0-1 factor used by stats.
// Chase / non-eligible scores never inflate rarity (cap at fair mid).
func PhotoQualityForRarity(r *PhotoScoreResult) float64 {
	if r == nil {
		return 0.5
	}
	if !r.RarityEligible || r.ChasePenalty {
		// Explicit: proximity/chase cannot raise rarity contribution
		return math.Min(r.Overall, 0.45)
	}
	return r.Overall
}

// DimensionValue returns a named dimension score.
func (d PhotoDimensionScores) DimensionValue(name string) float64 {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "stability":
		return d.Stability
	case "subject_completeness", "completeness":
		return d.SubjectCompleteness
	case "lighting":
		return d.Lighting
	case "occlusion":
		return d.Occlusion
	case "composition":
		return d.Composition
	case "safe_distance":
		return d.SafeDistance
	default:
		return 0
	}
}

// ThemeMet reports whether a score meets the daily theme (or a11y path).
func ThemeMet(theme PhotoDailyTheme, score *PhotoScoreResult, a11yCompleted bool) bool {
	if a11yCompleted {
		return true
	}
	if score == nil {
		return false
	}
	return score.Dimensions.DimensionValue(theme.TargetDimension) >= theme.TargetScore
}

func clamp01(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func clamp01Range(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// FormatOverallPercent helper for tips/UI.
func FormatOverallPercent(overall float64) string {
	return strconv.Itoa(int(math.Round(clamp01(overall) * 100)))
}
