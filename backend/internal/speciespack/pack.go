// Package speciespack 实现可扩展物种内容包与识别认证状态（AP-093）。
//
// 业务侧应通过 Registry 查询内容 ID / 版本与状态，避免在各模块维护物种 switch。
// 未通过黄金集认证或认证过期的物种仅可进入百科（catalog_only），不可捕获/发奖。
package speciespack

import "time"

// 识别/开放状态。
const (
	StatusCatalogOnly          = "catalog_only"
	StatusRecognitionCertified = "recognition_certified"
	StatusCapturable           = "capturable"
)

// 系统保留 ID（非内容包）。
const (
	IDUnknown     = "unknown"
	IDUnsupported = "unsupported"
)

// Localized 多语言字符串（至少建议提供 zh-CN / en）。
type Localized map[string]string

// Certification 黄金集认证元数据。
type Certification struct {
	// GoldenSetVersion 对应 vision golden manifest 版本。
	GoldenSetVersion string `json:"golden_set_version"`
	// ModelTrack 认证模型轨（detect 等）。
	ModelTrack string `json:"model_track,omitempty"`
	// CertifiedAt RFC3339；可空表示内置默认认证。
	CertifiedAt *time.Time `json:"certified_at,omitempty"`
	// ExpiresAt 过期后有效状态降级为 catalog_only。
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// Welfare 福利观察等级。
type Welfare struct {
	// Level: companion | wildlife | livestock | unknown
	Level string    `json:"level"`
	Notes Localized `json:"notes,omitempty"`
}

// Protection 保护状态。
type Protection struct {
	// Status: none | protected | endangered | unknown
	Status string    `json:"status"`
	Notes  Localized `json:"notes,omitempty"`
}

// Assets 展示素材引用（真实美术由 AP-110 流程补充）。
type Assets struct {
	Emoji           string `json:"emoji"`
	Icon            string `json:"icon,omitempty"`
	ThrowItemEmoji  string `json:"throw_item_emoji,omitempty"`
	PlaceholderTone string `json:"placeholder_tone,omitempty"`
}

// StatModifiers 战斗属性乘子与附加值。
type StatModifiers struct {
	HP   float64 `json:"hp"`
	ATK  float64 `json:"atk"`
	DEF  float64 `json:"def"`
	SPD  float64 `json:"spd"`
	Crit float64 `json:"crit"`
	Eva  float64 `json:"eva"`
}

// RarityWeight 稀有度权重项。
type RarityWeight struct {
	Tier   string  `json:"tier"`
	Weight float64 `json:"weight"`
}

// Gameplay 捕获/战斗可调参数（内容驱动，业务只读 ID）。
type Gameplay struct {
	ThrowItem        Localized      `json:"throw_item,omitempty"`
	CaptureMechanics Localized      `json:"capture_mechanics,omitempty"`
	ChargeRate       float64        `json:"charge_rate,omitempty"`
	OptimalRange     []float64      `json:"optimal_range,omitempty"` // [min,max] 0-100
	ChargeSpeed      float64        `json:"charge_speed,omitempty"`
	DetectThreshold  float64        `json:"detect_threshold,omitempty"`
	StatModifiers    *StatModifiers `json:"stat_modifiers,omitempty"`
	RarityWeights    []RarityWeight `json:"rarity_weights,omitempty"`
	BestRangeOffset  []float64      `json:"best_range_offset,omitempty"` // optional [dMin,dMax]
}

// Names 名称与别名。
type Names struct {
	Common     Localized `json:"common"`               // 俗名
	Scientific string    `json:"scientific,omitempty"` // 学名
	// Aliases 精确别名（大小写不敏感，匹配前 lower/trim）。
	Aliases []string `json:"aliases,omitempty"`
	// Contains 子串别名（用于中文等）。
	Contains []string `json:"contains,omitempty"`
	// ContainsExclude 命中 contains 时若同时包含则排除。
	ContainsExclude []string `json:"contains_exclude,omitempty"`
}

// Pack 物种内容包 schema。
type Pack struct {
	// ID 稳定内容 ID（如 cat）；业务与同步字段使用此值。
	ID string `json:"id"`
	// Version 内容包语义版本。
	Version string `json:"version"`
	// ContentID 全局内容引用（species.<id>）。
	ContentID string `json:"content_id"`
	// Status 声明状态；运行时以 EffectiveStatus 为准。
	Status string `json:"status"`
	// Certification 黄金集认证；capturable/recognition_certified 应提供。
	Certification *Certification `json:"certification,omitempty"`

	Names           Names                `json:"names"`
	Habitat         Localized            `json:"habitat,omitempty"`
	ObservationTips Localized            `json:"observation_tips,omitempty"`
	Welfare         Welfare              `json:"welfare"`
	Protection      Protection           `json:"protection"`
	Assets          Assets               `json:"assets"`
	Gameplay        Gameplay             `json:"gameplay,omitempty"`
	I18n            map[string]Localized `json:"i18n,omitempty"`
}

// Ref 业务交叉引用：内容 ID + 版本。
type Ref struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

// Ref 返回包引用。
func (p *Pack) Ref() Ref {
	if p == nil {
		return Ref{}
	}
	return Ref{ID: p.ID, Version: p.Version}
}

// LocalizedOr 取 locale，回退 zh-CN → en → 任意非空。
func LocalizedOr(m Localized, locale string) string {
	if m == nil {
		return ""
	}
	if locale != "" {
		if v := m[locale]; v != "" {
			return v
		}
	}
	if v := m["zh-CN"]; v != "" {
		return v
	}
	if v := m["en"]; v != "" {
		return v
	}
	for _, v := range m {
		if v != "" {
			return v
		}
	}
	return ""
}
