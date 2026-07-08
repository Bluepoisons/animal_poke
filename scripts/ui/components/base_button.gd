class_name AppButton
extends Button

## AppButton — 基础按钮组件
## 在全局 Theme 基础上, 预留点击音效(AudioManager)与防重复点击(防抖)。
## 调用方应连接 clicked 信号(带防抖), 而非原生 pressed。
## MVP 骨架: 音效资源待补(F2/美术阶段)。
##
## 用法:
##   var btn := AppButton.new()
##   btn.text = "捕获"
##   btn.clicked.connect(_on_capture)

signal clicked

const CLICK_LOCK_TIME := 0.2  # 防抖间隔(秒)

var _click_locked: bool = false


func _ready() -> void:
	pressed.connect(_on_pressed)


func _on_pressed() -> void:
	if _click_locked:
		return
	_click_locked = true
	# TODO(F2/美术): 接入点击音效。autoload 跨引用用运行时查找(F1 教训):
	# var am := get_node_or_null("/root/AudioManager")
	# if am != null and am.has_method("play_sfx"):
	#     am.call("play_sfx", _click_sfx)
	clicked.emit()
	get_tree().create_timer(CLICK_LOCK_TIME).timeout.connect(_unlock)


func _unlock() -> void:
	_click_locked = false
