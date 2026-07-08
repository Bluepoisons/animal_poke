class_name Toast
extends Control

## Toast — 轻量浮层提示组件
## 用 Toast.popup("消息") 在当前场景顶层显示短暂提示, 自动淡入淡出并销毁。
## 纯代码构建(无 .tscn 依赖), 通过 CanvasLayer 保证显示在最上层。
##
## 用法:
##   Toast.popup("已捕获!")
##   Toast.popup("金币不足", 3.0)

const DEFAULT_DURATION := 2.0
const FADE_TIME := 0.25

var _message: String = ""
var _duration: float = DEFAULT_DURATION
var _label: Label = null
var _timer: float = 0.0
var _fading_out: bool = false


func _ready() -> void:
	# 背景面板(撑满 Toast)
	var panel := Panel.new()
	panel.name = "Bg"
	panel.add_theme_stylebox_override("panel", _make_stylebox())
	add_child(panel)
	panel.set_anchors_preset(Control.PRESET_FULL_RECT)
	# 文字
	_label = Label.new()
	_label.name = "Label"
	_label.text = _message
	_label.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	_label.vertical_alignment = VERTICAL_ALIGNMENT_CENTER
	_label.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	_label.add_theme_font_size_override("font_size", 15)
	panel.add_child(_label)
	_label.set_anchors_preset(Control.PRESET_FULL_RECT)
	# 居中底部
	set_anchors_preset(Control.PRESET_CENTER_BOTTOM)
	modulate.a = 0.0


func _make_stylebox() -> StyleBoxFlat:
	var sb := StyleBoxFlat.new()
	sb.bg_color = Color(0, 0, 0, 0.78)
	sb.corner_radius_top_left = 10
	sb.corner_radius_top_right = 10
	sb.corner_radius_bottom_right = 10
	sb.corner_radius_bottom_left = 10
	sb.content_margin_left = 24.0
	sb.content_margin_top = 14.0
	sb.content_margin_right = 24.0
	sb.content_margin_bottom = 14.0
	return sb


func _process(delta: float) -> void:
	if _fading_out:
		modulate.a = maxf(0.0, modulate.a - delta / FADE_TIME)
		if modulate.a <= 0.0:
			queue_free()
		return
	# 淡入
	if modulate.a < 1.0:
		modulate.a = minf(1.0, modulate.a + delta / FADE_TIME)
	# 计时
	_timer += delta
	if _timer >= _duration:
		_fading_out = true


## 在当前场景顶层弹出 Toast。
static func popup(message: String, duration: float = DEFAULT_DURATION) -> void:
	var tree := Engine.get_main_loop() as SceneTree
	if tree == null or tree.current_scene == null:
		push_warning("[Toast] 无可用场景, 无法显示: " + message)
		return
	var toast := Toast.new()
	toast._message = message
	toast._duration = duration
	# CanvasLayer 保证显示在最上层
	var layer := CanvasLayer.new()
	layer.layer = 100
	tree.current_scene.add_child(layer)
	layer.add_child(toast)
	# Toast 销毁时一并移除 layer
	toast.tree_exiting.connect(layer.queue_free)
