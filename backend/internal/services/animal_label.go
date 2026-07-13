package services

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"animalpoke/backend/internal/taxonomy"
)

const maxConcreteAnimalLabelRunes = 24

var rejectedAnimalLabelExact = map[string]struct{}{
	"人": {}, "人类": {}, "人物": {}, "动物": {}, "生物": {},
	"桌子": {}, "石头": {}, "蘑菇": {}, "植物": {},
	"玩具": {}, "玩偶": {}, "模型": {}, "木马": {}, "木鱼": {},
	"车辆": {}, "汽车": {}, "机器人": {}, "屏幕": {},
	"野兽": {}, "猛兽": {}, "飞禽": {}, "家禽": {}, "家畜": {}, "鱼类": {}, "鸟类": {}, "昆虫": {}, "虫子": {},
}

var rejectedAnimalLabelFragments = []string{
	"桌子", "石头", "蘑菇", "玩具", "玩偶", "公仔", "模型", "雕像", "塑像", "摆件", "标本",
	"木马", "木鱼", "机器人", "机械", "卡通", "虚拟", "屏幕", "手机",
	"车辆", "汽车", "卡车", "火车", "自行车", "摩托车", "飞机",
	"人物", "行人", "男人", "女人", "男孩", "女孩", "儿童", "小孩", "婴儿",
	"植物", "真菌", "菌类", "食物", "水果", "蔬菜",
}

var concreteAnimalNameSuffixes = []string{
	"食蚁兽", "鸭嘴兽", "穿山甲", "长颈鹿", "黄鼠狼", "蝙蝠", "树懒", "犰狳", "考拉", "兔狲", "猞猁", "针鼹",
	"海豹", "海狮", "海象", "海牛", "儒艮", "海豚", "鲸", "豚",
	"刺猬", "鼹鼠", "狐狸", "獴", "鼬", "貂", "獾", "獭", "猬", "狸", "狲",
	"猫", "犬", "狗", "狼", "狐", "豺", "熊", "豹", "虎", "狮", "象", "鹿", "羚", "羊", "牛", "马", "驴", "驼", "猪", "猴", "猿", "鼠", "兔", "貘", "蝠",
	"鹦鹉", "企鹅", "鸵鸟", "鸸鹋", "戴胜", "犀鸟", "鸬鹚", "鹈鹕",
	"鸟", "鹰", "雕", "隼", "鹫", "鸮", "鹭", "鹤", "鹳", "鹮", "鸨", "雁", "鸭", "鹅", "鸡", "雉", "鸠", "鸽", "雀", "鸦", "鹊", "鹃", "鹂", "燕", "鸥", "鹱",
	"蝾螈", "蟾蜍", "蜥蜴", "壁虎", "娃娃鱼", "鳄鱼", "鲨鱼", "海马", "河豚",
	"蛇", "龟", "鳖", "鳄", "蛙", "鲵", "鱼", "鲨", "鳐", "鲼", "鳗", "鲤", "鲫", "鲢", "鳙", "鲶", "鳝", "鲈", "鲑", "鳟", "鲟", "鲱", "鲷", "鲳", "鲭", "鲅", "鲀", "鳅",
	"蚯蚓", "蚊子", "螳螂", "蟋蟀", "蚱蜢", "蜻蜓", "豆娘", "草蛉", "蟑螂", "甲虫", "瓢虫", "萤火虫", "蜉蝣", "蜘蛛", "蜈蚣", "马陆",
	"虫", "蚓", "蚊", "蝇", "蜂", "蚁", "虱", "蚤", "蛾", "蝶", "蝉", "蝗", "蚕", "蛭", "蝎", "虾", "蟹", "鲎",
	"鹦鹉螺", "章鱼", "乌贼", "鱿鱼", "墨鱼", "蜗牛", "蛞蝓", "牡蛎", "蛤蜊", "扇贝", "海螺", "藤壶",
	"螺", "贝", "蚌", "蛤", "水母", "海葵", "水螅", "珊瑚", "海绵", "海星", "海胆", "海参", "海鞘", "海百合",
}

// NormalizeConcreteAnimalLabel 将模型或用户给出的动物标签转为简体中文，
// 并返回其应使用的注册物种 ID；未注册但可信的真实动物返回 other_animal。
func NormalizeConcreteAnimalLabel(label string) (normalizedLabel, normalizedSpecies string, err error) {
	normalizedLabel, err = simplifyGeneratedChinese(strings.TrimSpace(label))
	if err != nil || !isChineseGeneratedText(normalizedLabel) || !isHanOnlyAnimalLabel(normalizedLabel) || isGenericOtherAnimalLabel(normalizedLabel) {
		return "", "", fmt.Errorf("animal label must be a concrete Simplified Chinese animal name")
	}
	if isRejectedAnimalLabel(normalizedLabel) {
		return "", "", fmt.Errorf("animal label is unsupported")
	}

	if normalized, _ := taxonomy.Normalize(normalizedLabel); normalized == taxonomy.SpeciesUnsupported {
		return "", "", fmt.Errorf("animal label is unsupported")
	}

	if species, ok := taxonomy.Registry().ResolveExactAlias(normalizedLabel); ok &&
		species != "other_animal" && taxonomy.Capturable(species) {
		return chineseSpecies(species), species, nil
	}
	if hasConcreteAnimalNameShape(normalizedLabel) {
		return normalizedLabel, "other_animal", nil
	}

	return "", "", fmt.Errorf("animal label is not a recognized concrete animal name")
}

// NormalizeAnimalIdentity validates and canonicalizes a species/Chinese-label
// pair at trust boundaries. Unlike ChineseSpeciesLabel, invalid labels never
// fall back to a display value.
func NormalizeAnimalIdentity(species, label string) (normalizedSpecies, normalizedLabel string, err error) {
	normalizedSpecies, _ = taxonomy.Normalize(strings.TrimSpace(species))
	if !taxonomy.Capturable(normalizedSpecies) {
		return "", "", fmt.Errorf("species is not capturable")
	}

	label = strings.TrimSpace(label)
	if label == "" {
		if normalizedSpecies == "other_animal" {
			return "", "", fmt.Errorf("species_label_zh is required for other_animal")
		}
		return normalizedSpecies, chineseSpecies(normalizedSpecies), nil
	}

	normalizedLabel, labelSpecies, labelErr := NormalizeConcreteAnimalLabel(label)
	if labelErr != nil {
		return "", "", labelErr
	}
	if labelSpecies != normalizedSpecies {
		return "", "", fmt.Errorf("species_label_zh does not match species")
	}
	return normalizedSpecies, normalizedLabel, nil
}

func isHanOnlyAnimalLabel(label string) bool {
	runeCount := utf8.RuneCountInString(label)
	if runeCount == 0 || runeCount > maxConcreteAnimalLabelRunes {
		return false
	}
	for _, r := range label {
		if !unicode.Is(unicode.Han, r) {
			return false
		}
	}
	return true
}

func isRejectedAnimalLabel(label string) bool {
	if _, rejected := rejectedAnimalLabelExact[label]; rejected {
		return true
	}
	for _, fragment := range rejectedAnimalLabelFragments {
		if strings.Contains(label, fragment) {
			return true
		}
	}
	return false
}

func hasConcreteAnimalNameShape(label string) bool {
	for _, suffix := range concreteAnimalNameSuffixes {
		if strings.HasSuffix(label, suffix) {
			return true
		}
	}
	return false
}

// ChineseSpeciesLabel returns the safe Simplified Chinese display name for a
// canonical species. A concrete lineage label wins only when it resolves back
// to the same species (or is a valid other_animal fallback label).
func ChineseSpeciesLabel(species, label string) string {
	canonical, _ := taxonomy.Normalize(species)
	if normalizedLabel, normalizedSpecies, err := NormalizeConcreteAnimalLabel(label); err == nil &&
		(normalizedSpecies == canonical || canonical == "other_animal" && normalizedSpecies == "other_animal") {
		return normalizedLabel
	}
	return chineseSpecies(canonical)
}

func isGenericOtherAnimalLabel(label string) bool {
	label = strings.TrimSpace(label)
	return strings.Contains(label, "动物") || strings.Contains(label, "生物")
}
