extends Node

## AudioManager — 音效与背景音乐管理(单例)
## MVP 阶段提供接口骨架, 具体音效资源在 F2/美术阶段补充。
## 总线布局(F2): Master / BGM / SFX。不存在的总线自动回退到 Master。

const BUS_MASTER := "Master"
const BUS_BGM := "BGM"
const BUS_SFX := "SFX"

var _bgm_player: AudioStreamPlayer = null


func _ready() -> void:
	_bgm_player = AudioStreamPlayer.new()
	_bgm_player.bus = _resolve_bus(BUS_BGM)
	add_child(_bgm_player)
	print("[AudioManager] 初始化完成 (骨架, 资源待 F2)")


## 播放背景音乐(单曲, 切换时自动停旧曲)
func play_bgm(stream: AudioStream) -> void:
	if stream == null:
		return
	if _bgm_player.stream == stream and _bgm_player.playing:
		return
	_bgm_player.stream = stream
	_bgm_player.play()


func stop_bgm() -> void:
	_bgm_player.stop()


## 播放一次性音效(播完自动回收节点)
func play_sfx(stream: AudioStream) -> void:
	if stream == null:
		return
	var p := AudioStreamPlayer.new()
	p.bus = _resolve_bus(BUS_SFX)
	p.stream = stream
	add_child(p)
	p.finished.connect(p.queue_free)
	p.play()


## 设置总线音量(linear 0~1)
func set_bus_volume(bus: String, linear: float) -> void:
	var idx := AudioServer.get_bus_index(bus)
	if idx < 0:
		return
	AudioServer.set_bus_volume_db(idx, linear_to_db(clampf(linear, 0.0, 1.0)))


## 总线存在则用原名, 否则回退 Master(避免运行时总线缺失警告)
func _resolve_bus(bus: String) -> String:
	if AudioServer.get_bus_index(bus) >= 0:
		return bus
	return BUS_MASTER
