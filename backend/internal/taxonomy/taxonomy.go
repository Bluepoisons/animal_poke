// Package taxonomy 服务端权威物种枚举与规范化。
// 捕获仅允许 cat|dog|goose；未知类映射为 unknown/unsupported，禁止默认鹅。
package taxonomy

import (
	"strings"
)

// 权威可捕获物种。
const (
	SpeciesCat         = "cat"
	SpeciesDog         = "dog"
	SpeciesGoose       = "goose"
	SpeciesUnknown     = "unknown"
	SpeciesUnsupported = "unsupported"
)

// Capturable 返回是否允许进入捕获/同步。
func Capturable(species string) bool {
	switch species {
	case SpeciesCat, SpeciesDog, SpeciesGoose:
		return true
	default:
		return false
	}
}

// IsAuthoritative 是否为权威枚举值（含 unknown/unsupported）。
func IsAuthoritative(species string) bool {
	switch species {
	case SpeciesCat, SpeciesDog, SpeciesGoose, SpeciesUnknown, SpeciesUnsupported:
		return true
	default:
		return false
	}
}

// Normalize 将模型/客户端原始标签规范为权威枚举。
// 规则：有限别名表；鸭/天鹅/鸟/人/玩偶等 → unsupported 或 unknown；绝不默认 goose。
// 返回 (normalized, originalLabelForAudit)。
func Normalize(raw string) (string, string) {
	original := strings.TrimSpace(raw)
	s := strings.ToLower(original)
	s = strings.ReplaceAll(s, "_", " ")
	s = strings.TrimSpace(s)
	if s == "" {
		return SpeciesUnknown, original
	}

	// 精确 / 明确别名（先匹配 unsupported，避免 "bird" 被误收）
	unsupportedExact := map[string]bool{
		"bird": true, "duck": true, "swan": true, "chicken": true, "rooster": true,
		"hen": true, "pigeon": true, "dove": true, "parrot": true, "eagle": true,
		"human": true, "person": true, "people": true, "man": true, "woman": true, "child": true, "baby": true,
		"toy": true, "doll": true, "plush": true, "statue": true, "screen": true, "phone": true,
		"car": true, "plant": true, "tree": true, "food": true,
		"鸟": true, "鸭": true, "鸭子": true, "天鹅": true, "鸡": true, "人": true, "人类": true,
		"玩偶": true, "玩具": true, "屏幕": true,
	}
	if unsupportedExact[s] {
		return SpeciesUnsupported, original
	}

	// 中文/英文子串：人像与非目标优先
	for _, kw := range []string{"human", "person", "people", "man ", "woman", "child", "baby", "人", "人类", "儿童"} {
		if strings.Contains(s, kw) {
			return SpeciesUnsupported, original
		}
	}
	for _, kw := range []string{"duck", "swan", "chicken", "pigeon", "parrot", "bird", "鸭", "天鹅", "鸟", "鸡"} {
		if strings.Contains(s, kw) {
			return SpeciesUnsupported, original
		}
	}
	for _, kw := range []string{"toy", "doll", "plush", "玩偶", "玩具", "screen", "屏幕"} {
		if strings.Contains(s, kw) {
			return SpeciesUnsupported, original
		}
	}

	// 猫
	if s == "cat" || s == "kitten" || s == "feline" || strings.Contains(s, "猫") {
		return SpeciesCat, original
	}
	if strings.Contains(s, "cat") && !strings.Contains(s, "cattle") && !strings.Contains(s, "caterpillar") {
		return SpeciesCat, original
	}

	// 狗
	if s == "dog" || s == "puppy" || s == "canine" || strings.Contains(s, "狗") || strings.Contains(s, "犬") {
		return SpeciesDog, original
	}
	if strings.Contains(s, "dog") {
		return SpeciesDog, original
	}

	// 鹅（仅明确鹅/goose/gander/gosling，不含 duck/swan/bird）
	if s == "goose" || s == "geese" || s == "gander" || s == "gosling" {
		return SpeciesGoose, original
	}
	if strings.Contains(s, "goose") || strings.Contains(s, "geese") || strings.Contains(s, "鹅") {
		// 排除 "mongoose" 等
		if strings.Contains(s, "mongoose") {
			return SpeciesUnsupported, original
		}
		return SpeciesGoose, original
	}

	// 其余未知：保留审计 label，不进捕获
	return SpeciesUnknown, original
}

// FilterCapturable 过滤并稳定排序：按 confidence desc，再按 species，再按原始顺序。
// 未知类保留在 outUnknown 供审计，不进入捕获列表。
type DetectLike struct {
	Species    string
	Confidence float64
	Label      string
	Index      int
}

func Partition(items []DetectLike) (capturable []DetectLike, auditOnly []DetectLike) {
	for _, it := range items {
		norm, orig := Normalize(it.Species)
		if it.Label == "" {
			it.Label = orig
		}
		it.Species = norm
		if Capturable(norm) {
			capturable = append(capturable, it)
		} else {
			auditOnly = append(auditOnly, it)
		}
	}
	// stable sort capturable by confidence desc, then species, then index
	for i := 0; i < len(capturable); i++ {
		for j := i + 1; j < len(capturable); j++ {
			a, b := capturable[i], capturable[j]
			swap := false
			if b.Confidence > a.Confidence {
				swap = true
			} else if b.Confidence == a.Confidence {
				if b.Species < a.Species {
					swap = true
				} else if b.Species == a.Species && b.Index < a.Index {
					swap = true
				}
			}
			if swap {
				capturable[i], capturable[j] = capturable[j], capturable[i]
			}
		}
	}
	return capturable, auditOnly
}
