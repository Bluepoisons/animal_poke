extends Node

## NetworkManager — 网络与在线状态管理(单例)
## 在线优先架构(4.3): 发现 / 捕获 / 数值生成必须联网, 断网仅图鉴浏览。
## 监听网络状态, 供发现/捕获模块订阅并做弱网降级(M5)。
##
## 状态: UNKNOWN(初始) / ONLINE / WEAK(弱网, 发现降级为扫描式单帧判定) /
## OFFLINE(断网, 仅图鉴浏览)。MVP 骨架, 心跳探测见 M5。

signal online()
signal weak_network()
signal offline()

enum NetState { UNKNOWN, ONLINE, WEAK, OFFLINE }

var _state: int = NetState.UNKNOWN


func _ready() -> void:
	# TODO(M5): 定时心跳探测网络质量, 据延迟/丢包区分 ONLINE/WEAK
	# TODO(M5): 接入弱网降级(扫描式)、断网续接逻辑
	# MVP 骨架: 初始 UNKNOWN, 待 M5 心跳判定真实状态
	print("[NetworkManager] 初始化完成 (骨架, 心跳探测待 M5)")


## 当前网络状态
func current_state() -> int:
	return _state


## 是否在线(含弱网)。断网时返回 false。
func is_online() -> bool:
	return _state == NetState.ONLINE or _state == NetState.WEAK


## 是否断网
func is_offline() -> bool:
	return _state == NetState.OFFLINE


## 是否弱网(发现应降级为扫描式单帧判定, 见 M5)
func is_weak() -> bool:
	return _state == NetState.WEAK


## 手动设置网络状态(供 M5 心跳调用, 或测试用)。
func set_state(s: int) -> void:
	if s == _state:
		return
	var prev := _state
	_state = s
	match s:
		NetState.ONLINE:
			online.emit()
		NetState.WEAK:
			weak_network.emit()
		NetState.OFFLINE:
			offline.emit()
	print("[NetworkManager] %s -> %s" % [NetState.keys()[prev], NetState.keys()[s]])
