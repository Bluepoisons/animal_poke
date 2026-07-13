package services

import (
	"fmt"
	"strings"
	"unicode"

	"animalpoke/backend/internal/speciespack"

	"github.com/longbridgeapp/opencc"
)

var traditionalToSimplified, traditionalToSimplifiedErr = opencc.New("t2s")

var chineseClassNames = map[string]string{
	"Warrior":  "战士",
	"Mage":     "法师",
	"Ranger":   "游侠",
	"Tank":     "守护者",
	"Support":  "辅助者",
	"Assassin": "影袭者",
}

var chineseElementNames = map[string]string{
	"Fire":     "火",
	"Water":    "水",
	"Grass":    "草木",
	"Electric": "雷电",
	"Ice":      "冰霜",
	"Dark":     "暗影",
	"Light":    "光明",
	"Earth":    "大地",
	"Wind":     "风",
}

var chineseBreedNames = map[string]string{
	"British Shorthair": "英国短毛猫",
	"Golden Retriever":  "金毛寻回犬",
	"Tabby":             "虎斑猫",
	"mix":               "混种",
	"mixed":             "混种",
}

var chineseColorNames = map[string]string{
	"blue-gray":  "蓝灰色",
	"golden":     "金色",
	"black":      "黑色",
	"brown":      "棕色",
	"orange":     "橘色",
	"white":      "白色",
	"soft-toned": "柔和色",
}

func containsHan(value string) bool {
	for _, r := range value {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func containsUnexpectedLetters(value string, allowed ...string) bool {
	for _, token := range allowed {
		if token != "" {
			value = strings.ReplaceAll(value, token, "")
		}
	}
	for _, r := range value {
		if unicode.IsLetter(r) && !unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func isChineseGeneratedText(value string, allowedLatin ...string) bool {
	if !containsHan(value) || containsUnexpectedLetters(value, allowedLatin...) {
		return false
	}
	normalized, err := simplifyGeneratedChinese(value)
	return err == nil && normalized == value
}

// simplifyGeneratedChinese normalizes model text before it is persisted or
// returned. Validation still rejects non-Chinese letters after conversion.
func simplifyGeneratedChinese(value string) (string, error) {
	if traditionalToSimplifiedErr != nil {
		return "", fmt.Errorf("initialize Simplified Chinese converter: %w", traditionalToSimplifiedErr)
	}
	normalized, err := traditionalToSimplified.Convert(value)
	if err != nil {
		return "", fmt.Errorf("convert generated text to Simplified Chinese: %w", err)
	}
	return normalized, nil
}

func chineseSpecies(value string) string {
	value = strings.TrimSpace(value)
	if pack, ok := speciespack.Default().Get(value); ok {
		if translated := speciespack.LocalizedOr(pack.Names.Common, "zh-CN"); translated != "" {
			return translated
		}
	}
	if containsHan(value) {
		return value
	}
	return "动物伙伴"
}

func chineseClass(value string) string {
	if translated := chineseClassNames[strings.TrimSpace(value)]; translated != "" {
		return translated
	}
	if containsHan(value) {
		return strings.TrimSpace(value)
	}
	return "旅者"
}

func chineseElement(value string) string {
	if translated := chineseElementNames[strings.TrimSpace(value)]; translated != "" {
		return translated
	}
	if containsHan(value) {
		return strings.TrimSpace(value)
	}
	return "星光"
}

func chineseBreed(value, species string) string {
	value = strings.TrimSpace(value)
	if translated := chineseBreedNames[value]; translated != "" {
		return translated
	}
	if containsHan(value) {
		return value
	}
	if value == "" {
		return chineseSpecies(species)
	}
	return "品种待确认"
}

func chineseColor(value string) string {
	value = strings.TrimSpace(value)
	if translated := chineseColorNames[value]; translated != "" {
		return translated
	}
	if containsHan(value) {
		return value
	}
	return "柔和色"
}
