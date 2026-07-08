class_name LoadingIndicator
extends Control

## LoadingIndicator — 加载提示组件
## 旋转弧线动画 + 可选文字, 纯代码构建(无 .tscn 依赖)。
##
## 用法: 实例化加入场景即自动播放; set_text("加载中") 改文字。

const SPIN_SPEED := 3.0  # 弧度/秒
const RADIUS := 24.0
const ARC_WIDTH := 4.0
const ACCENT_COLOR := Color(0.180, 0.659, 0.541, 1.0)

var _angle: float = 0.0
var _label: Label = null


func _ready() -> void:
	custom_minimum_size = Vector2(80, 80)
	_label = Label.new()
	_label.name = "Label"
	_label.text = "加载中"
	_label.add_theme_font_size_override("font_size", 13)
	_label.set_anchors_preset(Control.PRESET_CENTER_BOTTOM)
	_label.position = Vector2(-20, 44)
	add_child(_label)


func _process(delta: float) -> void:
	_angle += SPIN_SPEED * delta
	queue_redraw()


func _draw() -> void:
	var center := size / 2.0
	# 背景圆环(淡色)
	draw_arc(center, RADIUS, 0.0, TAU, 48, Color(1, 1, 1, 0.12), ARC_WIDTH)
	# 旋转弧(3/4 圈, 强调色)
	var points := PackedVector2Array()
	var segments := 24
	var arc_len := TAU * 0.75
	for i in range(segments + 1):
		var a := _angle + arc_len * float(i) / float(segments)
		points.append(center + Vector2(cos(a), sin(a)) * RADIUS)
	if points.size() >= 2:
		draw_polyline(points, ACCENT_COLOR, ARC_WIDTH, true)


func set_text(text: String) -> void:
	if _label:
		_label.text = text
