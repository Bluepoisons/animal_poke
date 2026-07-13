package speciespack

import (
	"fmt"
	"strings"
	"time"
)

// ValidatePack 校验 schema 必填字段与枚举。
func ValidatePack(p *Pack) error {
	if p == nil {
		return fmt.Errorf("pack is nil")
	}
	if strings.TrimSpace(p.ID) == "" {
		return fmt.Errorf("id required")
	}
	if p.ID == IDUnknown || p.ID == IDUnsupported {
		return fmt.Errorf("id %q is reserved", p.ID)
	}
	if strings.TrimSpace(p.Version) == "" {
		return fmt.Errorf("version required")
	}
	if strings.TrimSpace(p.ContentID) == "" {
		return fmt.Errorf("content_id required")
	}
	switch p.Status {
	case StatusCatalogOnly, StatusRecognitionCertified, StatusCapturable:
	default:
		return fmt.Errorf("invalid status %q", p.Status)
	}
	if LocalizedOr(p.Names.Common, "zh-CN") == "" && LocalizedOr(p.Names.Common, "en") == "" {
		return fmt.Errorf("names.common required")
	}
	if strings.TrimSpace(p.Welfare.Level) == "" {
		return fmt.Errorf("welfare.level required")
	}
	if strings.TrimSpace(p.Protection.Status) == "" {
		return fmt.Errorf("protection.status required")
	}
	if p.Protection.Status == "protected" || p.Protection.Status == "endangered" {
		if !strings.Contains(LocalizedOr(p.ObservationTips, "zh-CN"), "远距离") {
			return fmt.Errorf("protected species observation_tips must require remote observation")
		}
	}
	if strings.TrimSpace(p.Assets.Emoji) == "" {
		return fmt.Errorf("assets.emoji required")
	}
	if p.Status == StatusCapturable || p.Status == StatusRecognitionCertified {
		if p.Certification == nil || strings.TrimSpace(p.Certification.GoldenSetVersion) == "" {
			return fmt.Errorf("certification.golden_set_version required for status %s", p.Status)
		}
	}
	if p.Status == StatusCapturable {
		if err := validateCapturableGameplay(p); err != nil {
			return err
		}
	}
	if len(p.Gameplay.OptimalRange) > 0 && len(p.Gameplay.OptimalRange) != 2 {
		return fmt.Errorf("gameplay.optimal_range must be [min,max]")
	}
	return nil
}

func validateCapturableGameplay(p *Pack) error {
	if p.Gameplay.DetectThreshold <= 0 || p.Gameplay.DetectThreshold > 1 {
		return fmt.Errorf("gameplay.detect_threshold must be in (0,1]")
	}
	if p.Gameplay.StatModifiers == nil {
		return fmt.Errorf("gameplay.stat_modifiers required for capturable")
	}
	sm := p.Gameplay.StatModifiers
	if sm.HP <= 0 || sm.ATK <= 0 || sm.DEF <= 0 || sm.SPD <= 0 {
		return fmt.Errorf("gameplay.stat_modifiers hp/atk/def/spd must be > 0")
	}
	if len(p.Gameplay.RarityWeights) == 0 {
		return fmt.Errorf("gameplay.rarity_weights required for capturable")
	}
	return nil
}

// EffectiveStatus 应用认证过期与缺字段降级。
// - 认证过期 / 黄金集主版本不兼容 → catalog_only
// - capturable 缺关键 gameplay → recognition_certified（可识别不可捕）
// - 缺基础字段或认证元数据 → catalog_only
func EffectiveStatus(p *Pack, now time.Time, expectedGoldenVersion string) string {
	if p == nil {
		return StatusCatalogOnly
	}
	status := p.Status
	if status != StatusCapturable && status != StatusRecognitionCertified && status != StatusCatalogOnly {
		return StatusCatalogOnly
	}
	if err := validateBasePack(p); err != nil {
		return StatusCatalogOnly
	}
	if status == StatusCatalogOnly {
		return StatusCatalogOnly
	}

	if p.Certification == nil || strings.TrimSpace(p.Certification.GoldenSetVersion) == "" {
		return StatusCatalogOnly
	}
	if p.Certification.ExpiresAt != nil && !p.Certification.ExpiresAt.IsZero() {
		if now.After(*p.Certification.ExpiresAt) {
			return StatusCatalogOnly
		}
	}
	if expectedGoldenVersion != "" && p.Certification.GoldenSetVersion != "" {
		if !compatibleVersion(p.Certification.GoldenSetVersion, expectedGoldenVersion) {
			return StatusCatalogOnly
		}
	}

	if status == StatusCapturable {
		if err := validateCapturableGameplay(p); err != nil {
			// 认证仍有效时可保留 recognition_certified（可识别不可捕）
			return StatusRecognitionCertified
		}
	}
	return status
}

// validateBasePack 百科/认证共用的基础字段。
func validateBasePack(p *Pack) error {
	if p == nil {
		return fmt.Errorf("pack is nil")
	}
	if strings.TrimSpace(p.ID) == "" {
		return fmt.Errorf("id required")
	}
	if p.ID == IDUnknown || p.ID == IDUnsupported {
		return fmt.Errorf("id %q is reserved", p.ID)
	}
	if strings.TrimSpace(p.Version) == "" {
		return fmt.Errorf("version required")
	}
	if strings.TrimSpace(p.ContentID) == "" {
		return fmt.Errorf("content_id required")
	}
	if LocalizedOr(p.Names.Common, "zh-CN") == "" && LocalizedOr(p.Names.Common, "en") == "" {
		return fmt.Errorf("names.common required")
	}
	if strings.TrimSpace(p.Assets.Emoji) == "" {
		return fmt.Errorf("assets.emoji required")
	}
	return nil
}

// compatibleVersion 主版本一致即兼容（1.0.0 vs 1.2.3）。
func compatibleVersion(cert, expected string) bool {
	cm := major(cert)
	em := major(expected)
	if cm == "" || em == "" {
		return cert == expected
	}
	return cm == em
}

func major(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if i := strings.IndexByte(v, '.'); i >= 0 {
		return v[:i]
	}
	return v
}

// CanCapture 是否允许捕获/发奖。
func CanCapture(p *Pack, now time.Time, expectedGoldenVersion string) bool {
	return EffectiveStatus(p, now, expectedGoldenVersion) == StatusCapturable
}

// InEncyclopedia 是否可进入百科（所有有效内容包均可）。
func InEncyclopedia(p *Pack) bool {
	return p != nil && strings.TrimSpace(p.ID) != ""
}
