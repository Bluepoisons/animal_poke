// Package services — deterministic rarity/stats algorithm (AP-048).
// Rarity and battle stats are derived server-side from HMAC(seed_id|config_version)
// keyed by a server secret. LLM is only used for narrative text.
package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand/v2"
	"strings"
	"unicode"

	"animalpoke/backend/internal/config"
)

// StatsConfigVersion 写入 inference.config_version 的算法版本。
// 变更权重/映射时必须 bump，保证同 seed 在同版本下可复现。
const StatsConfigVersion = "stats-v1"

// Target rarity drop rates for typical mid-tier captures (docs 5.1).
// Used for distribution tests and inverse-CDF mapping.
var rarityCDF = []struct {
	rarity int
	cum    float64 // exclusive upper bound of cumulative mass
}{
	{1, 0.60}, // Common 60%
	{2, 0.85}, // Uncommon 25%
	{3, 0.95}, // Rare 10%
	{4, 0.99}, // Epic 4%
	{5, 1.00}, // Legendary 1%
}

// ValueFactors 生成依据：结果页/审计可展示的可解释因子。
type ValueFactors struct {
	PhotoQuality  float64 `json:"photo_quality"`  // 0-1 综合拍摄质量
	Completeness  float64 `json:"completeness"`   // 0-1 主体完整度
	SpeciesWeight float64 `json:"species_weight"` // 0-1 物种规则权重
	BreedWeight   float64 `json:"breed_weight"`   // 0-1 品种权重
	ColorWeight   float64 `json:"color_weight"`   // 0-1 毛色权重
	BaseScore     float64 `json:"base_score"`     // 0-1 基础分
	RandomJitter  float64 `json:"random_jitter"`  // 有限随机扰动（来自 seed）
	FinalScore    float64 `json:"final_score"`    // 0-1 映射用最终分
	Roll          float64 `json:"roll"`           // seed 派生的 [0,1) 均匀量
	QualityBand   string  `json:"quality_band"`   // excellent|good|fair|poor
	ConfigVersion string  `json:"config_version"`
	SeedID        string  `json:"seed_id,omitempty"`
}

// ComputeDeterministicValue 用 HMAC 派生 RNG，计算 rarity/stats（不含 narrative）。
// seedID 应为 capture/parent inference id；secret 为 STATS_HMAC_KEY（与 JWT 独立）。
func ComputeDeterministicValue(input ValueInput, seedID, secret, configVersion string) *ValueResult {
	if configVersion == "" {
		configVersion = StatsConfigVersion
	}
	if secret == "" {
		// 开发兜底；production 由 config.Validate 强制配置 STATS_HMAC_KEY
		secret = config.DefaultDevStatsHMACKey
	}
	if seedID == "" {
		// 无稳定 id 时用输入摘要，保证同输入同结果（开发/mock 兜底）
		seedID = "input:" + inputDigest(input)
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(seedID))
	mac.Write([]byte("|"))
	mac.Write([]byte(configVersion))
	digest := mac.Sum(nil)

	var chaSeed [32]byte
	copy(chaSeed[:], digest)
	rng := rand.New(rand.NewChaCha8(chaSeed))

	// ---- 因子 ----
	completeness := quality01(input.SubjectCompleteness)
	photoQuality := averageQuality01(input)
	speciesW := speciesWeight(input.Species)
	breedW := breedWeight(input.Breed)
	colorW := colorWeight(input.Color)

	// docs 5.3: base = breed*0.35 + color*0.25 + quality*0.25 + species*0.15
	base := breedW*0.35 + colorW*0.25 + photoQuality*0.25 + speciesW*0.15
	// 有限随机：±0.1，来自 seed
	jitter := (rng.Float64()*2 - 1) * 0.1
	// 均匀 roll + 因子偏移：高因子 → 更稀有（roll 下移）
	roll := rng.Float64()
	factorBonus := (base - 0.40) * 0.30
	adjusted := clampFloat(roll-factorBonus+jitter*0.15, 0, 0.999999)
	rarity := rarityFromRoll(adjusted)

	// 质量过差：稀有度上限降一级（docs 5.3）
	band := qualityBand(photoQuality)
	if band == "poor" && rarity > 1 {
		rarity--
	}

	// ---- 六维属性 ----
	// 质量对属性的倍率
	statMul := qualityStatMultiplier(band)
	// 物种基础偏向
	hpBase, atkBase, defBase, spdBase := speciesStatBases(input.Species, input.BodyType)

	// 稀有度抬升
	rarityBoost := float64(rarity-1) * 0.06

	// 个体 ±15% 浮动（seed 有限随机）
	indiv := func() float64 { return 0.85 + rng.Float64()*0.30 }

	hp := clampInt(int(math.Round((hpBase * (1 + rarityBoost) * statMul * indiv()))), 10, 100)
	atk := clampInt(int(math.Round((atkBase * (1 + rarityBoost) * statMul * indiv()))), 5, 50)
	def := clampInt(int(math.Round((defBase * (1 + rarityBoost) * statMul * indiv()))), 5, 50)
	spd := clampInt(int(math.Round((spdBase * (1 + rarityBoost) * statMul * indiv()))), 5, 50)

	classes := []string{"Warrior", "Mage", "Ranger", "Tank", "Support", "Assassin"}
	elements := []string{"Fire", "Water", "Grass", "Electric", "Ice", "Dark", "Light", "Earth", "Wind"}
	// 物种偏好 class/element，再由 seed 微调
	class := preferredClass(input.Species, input.BodyType, classes, rng)
	element := preferredElement(input.Species, elements, rng)

	finalScore := clampFloat(base+jitter, 0, 1)
	factors := &ValueFactors{
		PhotoQuality:  round3(photoQuality),
		Completeness:  round3(completeness),
		SpeciesWeight: round3(speciesW),
		BreedWeight:   round3(breedW),
		ColorWeight:   round3(colorW),
		BaseScore:     round3(base),
		RandomJitter:  round3(jitter),
		FinalScore:    round3(finalScore),
		Roll:          round3(roll),
		QualityBand:   band,
		ConfigVersion: configVersion,
		SeedID:        seedID,
	}

	return &ValueResult{
		Rarity:        rarity,
		HP:            hp,
		ATK:           atk,
		DEF:           def,
		SPD:           spd,
		Class:         class,
		Element:       element,
		Factors:       factors,
		ConfigVersion: configVersion,
		SeedID:        seedID,
	}
}

// narrativeFallback 无 LLM 时的确定性叙事模板（不含随机）。
func narrativeFallback(input ValueInput, v *ValueResult) string {
	breed := input.Breed
	if breed == "" {
		breed = "wild"
	}
	color := input.Color
	if color == "" {
		color = "natural"
	}
	return fmt.Sprintf(
		"A %s %s discovered in the wild. Its %s coat gleams with potential. Classed as %s/%s with ★%d power.",
		breed, input.Species, color, v.Class, v.Element, v.Rarity,
	)
}

func inputDigest(input ValueInput) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s|%s|%s|%s|%d|%d|%d|%d|%d|%d",
		input.Species, input.Breed, input.Color, input.BodyType,
		input.SubjectCompleteness, input.Clarity, input.Lighting,
		input.Composition, input.Pose, input.Angle,
	)
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}

func quality01(v int) float64 {
	if v <= 0 {
		return 0.5
	}
	if v > 10 {
		v = 10
	}
	return float64(v) / 10.0
}

func averageQuality01(input ValueInput) float64 {
	vals := []int{
		input.SubjectCompleteness, input.Clarity, input.Lighting,
		input.Composition, input.Pose, input.Angle,
	}
	sum := 0
	n := 0
	for _, v := range vals {
		if v > 0 {
			sum += v
			n++
		}
	}
	if n == 0 {
		return 0.5
	}
	return float64(sum) / float64(n) / 10.0
}

func qualityBand(q float64) string {
	// q is 0-1; map to docs bands on 0-10 scale
	s := q * 10
	switch {
	case s >= 8:
		return "excellent"
	case s >= 5:
		return "good"
	case s >= 3:
		return "fair"
	default:
		return "poor"
	}
}

func qualityStatMultiplier(band string) float64 {
	switch band {
	case "excellent":
		return 1.10
	case "good":
		return 1.00
	case "fair":
		return 0.90
	default:
		return 0.75
	}
}

func speciesWeight(species string) float64 {
	switch strings.ToLower(strings.TrimSpace(species)) {
	case "cat":
		return 0.42
	case "dog":
		return 0.48
	case "goose":
		return 0.55
	default:
		return 0.40
	}
}

func breedWeight(breed string) float64 {
	b := strings.ToLower(strings.TrimSpace(breed))
	if b == "" || b == "unknown" || b == "test" {
		return 0.30
	}
	// 稀有关键词抬升
	rareHints := []string{"persian", "siamese", "maine", "sphynx", "husky", "akita", "samoyed", "bengal", "ragdoll"}
	for _, h := range rareHints {
		if strings.Contains(b, h) {
			return 0.72
		}
	}
	// 常见
	commonHints := []string{"tabby", "mixed", "mongrel", "domestic", "shorthair", "british"}
	for _, h := range commonHints {
		if strings.Contains(b, h) {
			return 0.28
		}
	}
	// 稳定哈希到 0.25-0.65
	sum := 0
	for _, r := range b {
		if unicode.IsLetter(r) {
			sum += int(r)
		}
	}
	return 0.25 + float64(sum%41)/100.0
}

func colorWeight(color string) float64 {
	c := strings.ToLower(strings.TrimSpace(color))
	if c == "" || c == "unknown" || c == "test" {
		return 0.30
	}
	rare := []string{"calico", "tortoiseshell", "merle", "albino", "blue-point", "chocolate", "lilac"}
	for _, h := range rare {
		if strings.Contains(c, h) {
			return 0.70
		}
	}
	common := []string{"black", "white", "brown", "orange", "gray", "grey", "blue-gray", "yellow"}
	for _, h := range common {
		if strings.Contains(c, h) {
			return 0.32
		}
	}
	sum := 0
	for _, r := range c {
		sum += int(r)
	}
	return 0.28 + float64(sum%35)/100.0
}

func rarityFromRoll(u float64) int {
	for _, b := range rarityCDF {
		if u < b.cum {
			return b.rarity
		}
	}
	return 5
}

func speciesStatBases(species, bodyType string) (hp, atk, def, spd float64) {
	// mid-range bases within clamp windows
	switch strings.ToLower(strings.TrimSpace(species)) {
	case "cat":
		hp, atk, def, spd = 48, 22, 16, 32
	case "dog":
		hp, atk, def, spd = 62, 28, 22, 22
	case "goose":
		hp, atk, def, spd = 55, 20, 30, 18
	default:
		hp, atk, def, spd = 50, 20, 20, 20
	}
	bt := strings.ToLower(bodyType)
	switch {
	case strings.Contains(bt, "sturdy") || strings.Contains(bt, "stocky") || strings.Contains(bt, "tank"):
		hp *= 1.10
		def *= 1.10
		spd *= 0.92
	case strings.Contains(bt, "slim") || strings.Contains(bt, "lean") || strings.Contains(bt, "agile"):
		spd *= 1.12
		hp *= 0.94
	}
	return
}

func preferredClass(species, bodyType string, classes []string, rng *rand.Rand) string {
	bt := strings.ToLower(bodyType)
	if strings.Contains(bt, "sturdy") || strings.Contains(bt, "stocky") {
		if rng.Float64() < 0.55 {
			return "Tank"
		}
	}
	switch strings.ToLower(species) {
	case "cat":
		if rng.Float64() < 0.45 {
			return "Assassin"
		}
		if rng.Float64() < 0.5 {
			return "Ranger"
		}
	case "dog":
		if rng.Float64() < 0.45 {
			return "Warrior"
		}
		if rng.Float64() < 0.5 {
			return "Tank"
		}
	case "goose":
		if rng.Float64() < 0.4 {
			return "Support"
		}
		if rng.Float64() < 0.5 {
			return "Mage"
		}
	}
	return classes[rng.IntN(len(classes))]
}

func preferredElement(species string, elements []string, rng *rand.Rand) string {
	switch strings.ToLower(species) {
	case "cat":
		if rng.Float64() < 0.35 {
			return "Wind"
		}
		if rng.Float64() < 0.4 {
			return "Dark"
		}
	case "dog":
		if rng.Float64() < 0.35 {
			return "Earth"
		}
		if rng.Float64() < 0.4 {
			return "Fire"
		}
	case "goose":
		if rng.Float64() < 0.35 {
			return "Water"
		}
		if rng.Float64() < 0.4 {
			return "Ice"
		}
	}
	return elements[rng.IntN(len(elements))]
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func round3(v float64) float64 {
	return math.Round(v*1000) / 1000
}

// seedUint64 保留工具：从 digest 取两个 uint64（测试/扩展用）。
func seedUint64(digest []byte) (uint64, uint64) {
	if len(digest) < 16 {
		padded := make([]byte, 16)
		copy(padded, digest)
		digest = padded
	}
	return binary.BigEndian.Uint64(digest[0:8]), binary.BigEndian.Uint64(digest[8:16])
}
