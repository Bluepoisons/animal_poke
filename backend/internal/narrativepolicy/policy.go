// Package narrativepolicy validates AI narrative inputs and outputs before publication.
package narrativepolicy

import (
	"regexp"
	"strings"
	"unicode"
)

// Violation identifies a policy class without retaining sensitive source text.
type Violation struct {
	Rule string
}

func (v Violation) Error() string { return "narrative policy violation: " + v.Rule }

type rule struct {
	name    string
	pattern *regexp.Regexp
}

// outputRules cover claims that can make a fictional vignette look like a
// statement about a real animal, unsafe interactions, or an exploitative reward.
var outputRules = []rule{
	{name: "real_identity_or_owner", pattern: regexp.MustCompile(`(?i)\b(?:owned by|owner is|lives at|diagnosed|medical record)\b|主人是|病历|确诊|真实.{0,12}(?:经历|情绪|健康|主人|意图)`)},
	{name: "precise_location", pattern: regexp.MustCompile(`(?i)\b(?:gps|latitude|longitude|coordinates?|address)\b|(?:经纬度|精确(?:地址|位置|坐标)|家住|住在)`)},
	{name: "unsafe_interaction", pattern: regexp.MustCompile(`(?i)\b(?:chase|feed|touch|trespass|night adventure)\b|(?:追逐|投喂|触摸|私闯|夜间冒险)`)},
	{name: "emergency_reward", pattern: regexp.MustCompile(`(?i)(?:lost|injured|stray|emergency).{0,48}(?:rare|reward|bonus)|(?:走失|受伤|流浪|紧急).{0,48}(?:稀有|奖励|加成)`)},
	{name: "paid_moral_choice", pattern: regexp.MustCompile(`(?i)(?:pay|purchase|premium).{0,32}(?:choice|moral)|(?:付费|购买|高级).{0,32}(?:选择|道德)`)},
	{name: "minor_inequity", pattern: regexp.MustCompile(`(?i)(?:minor|child|accessibility|home).{0,48}(?:paywall|premium|locked)|(?:未成年人|儿童|无障碍|居家).{0,48}(?:付费墙|高级|锁定)`)},
}

var promptInjectionRules = []rule{
	{name: "prompt_injection", pattern: regexp.MustCompile(`(?is)(?:ignore\s+(?:all\s+)?(?:previous\s+)?(?:instructions|rules)|system\s*:|assistant\s*:|developer\s*:|忽略.{0,24}(?:指令|规则)|系统\s*[:：]|助手\s*[:：])`)},
}

var sensitiveScenarioPattern = regexp.MustCompile(`(?i)\b(?:lost|injured|stray|emergency)\b|(?:走失|受伤|流浪|紧急)`)

const safetyGuidance = "【安全指引】保持安全距离，联系当地动物救助机构；此类情况不改变游戏进度。"

// ValidateOutput rejects content that cannot safely be published as a fictional vignette.
func ValidateOutput(text string) error {
	return validate(text, outputRules)
}

// SafetyGuidance replaces emergency-oriented fiction with a non-rewarding
// action guide. The boolean is true only when the source mentioned a sensitive scenario.
func SafetyGuidance(text string) (string, bool) {
	if sensitiveScenarioPattern.MatchString(text) {
		return safetyGuidance, true
	}
	return "", false

}

// ValidatePromptInput rejects instruction-shaped data before it can reach the LLM.
func ValidatePromptInput(fields ...string) error {
	for _, field := range fields {
		if strings.IndexFunc(field, unicode.IsControl) >= 0 {
			return Violation{Rule: "prompt_control_character"}
		}
		if err := validate(field, promptInjectionRules); err != nil {
			return err
		}
	}
	return nil
}

func validate(text string, rules []rule) error {
	for _, rule := range rules {
		if rule.pattern.MatchString(text) {
			return Violation{Rule: rule.name}
		}
	}
	return nil
}
