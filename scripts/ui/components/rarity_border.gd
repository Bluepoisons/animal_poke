class_name RarityBorder
extends Panel

## RarityBorder — 稀有度边框组件
## 根据稀有度显示对应颜色边框(5.1: 灰/绿/蓝/紫/金), 传说级预留粒子特效位。
## 背景透明, 仅作边框装饰, 适合叠加在卡片/图鉴条目之上。
##
## 用法: 实例化后 set_rarity(Rarity.Tier.RARE) -> 蓝色边框

var _tier: int = 0


func _ready() -> void:
	_apply_border(_tier)


## 设置稀有度并刷新边框
func set_rarity(tier: int) -> void:
	_tier = tier
	_apply_border(tier)


func get_rarity() -> int:
	return _tier


func _apply_border(tier: int) -> void:
	var sb := StyleBoxFlat.new()
	sb.bg_color = Color(0, 0, 0, 0)  # 透明背景, 仅边框
	sb.border_width_left = 3
	sb.border_width_top = 3
	sb.border_width_right = 3
	sb.border_width_bottom = 3
	sb.border_color = Rarity.color_of(tier)
	sb.corner_radius_top_left = 8
	sb.corner_radius_top_right = 8
	sb.corner_radius_bottom_right = 8
	sb.corner_radius_bottom_left = 8
	sb.content_margin_left = 2.0
	sb.content_margin_top = 2.0
	sb.content_margin_right = 2.0
	sb.content_margin_bottom = 2.0
	add_theme_stylebox_override("panel", sb)
	if Rarity.has_particle(tier):
		_setup_particles()


func _setup_particles() -> void:
	# TODO(美术阶段): 传说级粒子特效(GPUParticles2D + 金色粒子材质)
	# 当前预留, 不影响边框显示
	pass
