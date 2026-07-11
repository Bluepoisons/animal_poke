// Package taxonomy 服务端权威物种规范化（AP-007 / AP-093）。
// 捕获资格与别名解析由 speciespack 内容注册表驱动，禁止硬编码多处 switch。
// 未知类映射为 unknown/unsupported，禁止默认鹅。
package taxonomy

import (
	"animalpoke/backend/internal/speciespack"
)

// 权威系统 ID（与内容包 ID 并存）。
const (
	SpeciesCat         = "cat"
	SpeciesDog         = "dog"
	SpeciesGoose       = "goose"
	SpeciesRabbit      = "rabbit" // 试点百科物种（catalog_only）
	SpeciesUnknown     = speciespack.IDUnknown
	SpeciesUnsupported = speciespack.IDUnsupported
)

// Registry 返回默认物种内容注册表。
func Registry() *speciespack.Registry {
	return speciespack.Default()
}

// Capturable 返回是否允许进入捕获/同步/发奖。
// 仅 recognition 认证且状态为 capturable、且未降级的内容包返回 true。
func Capturable(species string) bool {
	return Registry().Capturable(species)
}

// IsAuthoritative 是否为权威枚举值（内容包 ID 或 unknown/unsupported）。
func IsAuthoritative(species string) bool {
	return Registry().IsKnown(species)
}

// Normalize 将模型/客户端原始标签规范为内容 ID 或 unknown/unsupported。
// 返回 (normalized, originalLabelForAudit)。
func Normalize(raw string) (string, string) {
	return Registry().Normalize(raw)
}

// CapturableSpecies 当前可捕获内容 ID 列表（稳定排序）。
func CapturableSpecies() []string {
	return Registry().CapturableIDs()
}

// EncyclopediaSpecies 百科可见内容 ID（含未认证试点）。
func EncyclopediaSpecies() []string {
	return Registry().EncyclopediaIDs()
}

// EffectiveStatus 查询物种有效认证状态。
func EffectiveStatus(species string) string {
	return Registry().EffectiveStatusOf(species)
}

// ContentRef 返回内容引用；未知 ID 返回空版本。
func ContentRef(species string) speciespack.Ref {
	if p, ok := Registry().Get(species); ok {
		return p.Ref()
	}
	return speciespack.Ref{ID: species}
}

// DetectLike 检测条目（用于分区排序）。
type DetectLike struct {
	Species    string
	Confidence float64
	Label      string
	Index      int
}

// Partition 过滤并稳定排序：按 confidence desc，再按 species，再按原始顺序。
// 未知/未认证类保留在 auditOnly 供审计与百科，不进入捕获列表。
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
	for i := range capturable {
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
