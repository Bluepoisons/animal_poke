extends GdUnitTestSuite

# AudioManager 测试: _resolve_bus 回退 + set_bus_volume(用 Master 总线, 测后还原)。
# play_bgm/play_sfx 的 null 守卫。音频资源播放不在单元测试覆盖(无音频设备/资源)。

const AM_SCRIPT := "res://scripts/autoload/audio_manager.gd"

var _saved_master_vol_db: float = 0.0


func before_test() -> void:
	var idx := AudioServer.get_bus_index("Master")
	if idx >= 0:
		_saved_master_vol_db = AudioServer.get_bus_volume_db(idx)


func after_test() -> void:
	var idx := AudioServer.get_bus_index("Master")
	if idx >= 0:
		AudioServer.set_bus_volume_db(idx, _saved_master_vol_db)


func _new_am() -> Node:
	var am: Node = auto_free(load(AM_SCRIPT).new())
	add_child(am)
	return am


func test_resolve_bus_falls_back_to_master() -> void:
	var am := _new_am()
	# 默认项目只有 Master 总线, BGM/SFX 不存在 → 回退 Master
	assert_str(am._resolve_bus("Master")).is_equal("Master")
	assert_str(am._resolve_bus("BGM")).is_equal("Master")
	assert_str(am._resolve_bus("SFX")).is_equal("Master")
	assert_str(am._resolve_bus("Nonexistent")).is_equal("Master")


func test_set_bus_volume_on_master() -> void:
	if AudioServer.get_bus_index("Master") < 0:
		print("[audio_manager] 无 Master 总线, 跳过")
		return
	var am := _new_am()
	am.set_bus_volume("Master", 0.5)
	var idx := AudioServer.get_bus_index("Master")
	# 用容差比较(AudioServer 内部可能做精度转换)
	assert_bool(absf(AudioServer.get_bus_volume_db(idx) - linear_to_db(0.5)) < 0.001).is_true()


func test_set_bus_volume_clamps_above_one() -> void:
	if AudioServer.get_bus_index("Master") < 0:
		print("[audio_manager] 无 Master 总线, 跳过")
		return
	var am := _new_am()
	am.set_bus_volume("Master", 5.0)  # clamp 到 1.0
	var idx := AudioServer.get_bus_index("Master")
	assert_bool(absf(AudioServer.get_bus_volume_db(idx) - linear_to_db(1.0)) < 0.001).is_true()


func test_set_bus_volume_unknown_bus_no_crash() -> void:
	var am := _new_am()
	# 未知总线 idx < 0, 早返回, 不崩溃
	am.set_bus_volume("TotallyUnknownBus", 0.5)


func test_play_bgm_null_is_noop() -> void:
	var am := _new_am()
	am.play_bgm(null)  # null 守卫, 不崩溃


func test_play_sfx_null_is_noop() -> void:
	var am := _new_am()
	am.play_sfx(null)
