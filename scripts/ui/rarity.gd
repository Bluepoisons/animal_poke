class_name Rarity
extends RefCounted

## Rarity — 稀有度枚举与视觉标识颜色
## 参考: 游戏开发计划.md 5.1 稀有度体系
## 5 级: Common(灰) / Uncommon(绿) / Rare(蓝) / Epic(紫) / Legendary(金 + 粒子特效)
##
## 用法: Rarity.color_of(Rarity.Tier.RARE) -> 蓝色

enum Tier {
	COMMON,     # 1 普通  灰色边框
	UNCOMMON,   # 2 非凡  绿色边框
	RARE,       # 3 稀有  蓝色边框
	EPIC,       # 4 史诗  紫色边框
	LEGENDARY,  # 5 传说  金色边框 + 粒子特效
}

# 边框色(与 5.1 表严格一致)
const COLOR_COMMON := Color(0.55, 0.55, 0.55, 1.0)      # 灰
const COLOR_UNCOMMON := Color(0.30, 0.80, 0.30, 1.0)    # 绿
const COLOR_RARE := Color(0.30, 0.55, 1.00, 1.0)        # 蓝
const COLOR_EPIC := Color(0.65, 0.35, 0.95, 1.0)        # 紫
const COLOR_LEGENDARY := Color(1.00, 0.80, 0.20, 1.0)   # 金

# 显示名(中文)
const NAME_COMMON := "普通"
const NAME_UNCOMMON := "非凡"
const NAME_RARE := "稀有"
const NAME_EPIC := "史诗"
const NAME_LEGENDARY := "传说"

# 基础掉率(5.1, 供 UI 展示)
const RATE_COMMON := 0.60
const RATE_UNCOMMON := 0.25
const RATE_RARE := 0.10
const RATE_EPIC := 0.04
const RATE_LEGENDARY := 0.01


## 稀有度对应的边框色
static func color_of(tier: int) -> Color:
	match tier:
		Tier.COMMON:
			return COLOR_COMMON
		Tier.UNCOMMON:
			return COLOR_UNCOMMON
		Tier.RARE:
			return COLOR_RARE
		Tier.EPIC:
			return COLOR_EPIC
		Tier.LEGENDARY:
			return COLOR_LEGENDARY
		_:
			push_warning("[Rarity] 未知稀有度: %d, 回退灰色" % tier)
			return COLOR_COMMON


## 稀有度中文名
static func name_of(tier: int) -> String:
	match tier:
		Tier.COMMON:
			return NAME_COMMON
		Tier.UNCOMMON:
			return NAME_UNCOMMON
		Tier.RARE:
			return NAME_RARE
		Tier.EPIC:
			return NAME_EPIC
		Tier.LEGENDARY:
			return NAME_LEGENDARY
		_:
			return "未知"


## 基础掉率(0~1)
static func rate_of(tier: int) -> float:
	match tier:
		Tier.COMMON:
			return RATE_COMMON
		Tier.UNCOMMON:
			return RATE_UNCOMMON
		Tier.RARE:
			return RATE_RARE
		Tier.EPIC:
			return RATE_EPIC
		Tier.LEGENDARY:
			return RATE_LEGENDARY
		_:
			return 0.0


## 是否带粒子特效(仅传说级)
static func has_particle(tier: int) -> bool:
	return tier == Tier.LEGENDARY
