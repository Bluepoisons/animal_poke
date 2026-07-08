class_name BasePanel
extends Panel

## BasePanel — 基础面板组件
## 自动继承全局 Theme(project.godot theme/custom)的 panel 样式, 提供可选标题。
##
## 用法: 实例化后 set_title("标题"); 内容节点直接 add_child 添加。

var _title_label: Label = null


func _ready() -> void:
	# Panel 自动继承全局 Theme。
	# 若 Theme 未全局注册, 取消下行注释强制应用 default_theme 的 panel 样式:
	# var t := load("res://themes/default_theme.tres") as Theme
	# if t != null:
	#     add_theme_stylebox_override("panel", t.get_stylebox("panel", "Panel"))
	pass


## 设置标题(左上角)
func set_title(text: String) -> void:
	if _title_label == null:
		_title_label = Label.new()
		_title_label.name = "TitleLabel"
		_title_label.position = Vector2(12, 8)
		_title_label.add_theme_font_size_override("font_size", 18)
		add_child(_title_label)
	_title_label.text = text


func get_title_label() -> Label:
	return _title_label
