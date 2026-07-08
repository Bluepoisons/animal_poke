extends GdUnitTestSuite

# NetworkManager 测试: 状态机 + 信号。实例化隔离(不依赖全局 autoload)。
# Godot 4.7: enum 走 preload 的脚本, 实例走 Variant 动态分发。

const NMScript := preload("res://scripts/autoload/network_manager.gd")


func _new_nm() -> Variant:
	return auto_free(NMScript.new())


func test_initial_state() -> void:
	var nm: Variant = _new_nm()
	assert_int(nm.current_state()).is_equal(NMScript.NetState.UNKNOWN)
	assert_bool(nm.is_online()).is_false()
	assert_bool(nm.is_offline()).is_false()
	assert_bool(nm.is_weak()).is_false()


func test_set_state_online() -> void:
	var nm: Variant = _new_nm()
	var emitter := monitor_signals(nm)
	nm.set_state(NMScript.NetState.ONLINE)
	await assert_signal(emitter).is_emitted("online")
	assert_bool(nm.is_online()).is_true()
	assert_bool(nm.is_offline()).is_false()
	assert_bool(nm.is_weak()).is_false()


func test_set_state_weak() -> void:
	var nm: Variant = _new_nm()
	var emitter := monitor_signals(nm)
	nm.set_state(NMScript.NetState.WEAK)
	await assert_signal(emitter).is_emitted("weak_network")
	assert_bool(nm.is_weak()).is_true()
	assert_bool(nm.is_online()).is_true()  # 弱网仍算在线


func test_set_state_offline() -> void:
	var nm: Variant = _new_nm()
	var emitter := monitor_signals(nm)
	nm.set_state(NMScript.NetState.OFFLINE)
	await assert_signal(emitter).is_emitted("offline")
	assert_bool(nm.is_offline()).is_true()
	assert_bool(nm.is_online()).is_false()


func test_same_state_does_not_emit() -> void:
	var nm: Variant = _new_nm()
	nm.set_state(NMScript.NetState.ONLINE)
	var emitter := monitor_signals(nm)
	nm.set_state(NMScript.NetState.ONLINE)  # 同状态, 不发信号
	await await_millis(30)
	assert_signal(emitter).is_not_emitted("online")
	assert_signal(emitter).is_not_emitted("offline")
	assert_signal(emitter).is_not_emitted("weak_network")
	assert_int(nm.current_state()).is_equal(NMScript.NetState.ONLINE)


func test_state_transitions_emit_correct_signal() -> void:
	var nm: Variant = _new_nm()
	var emitter := monitor_signals(nm)
	nm.set_state(NMScript.NetState.ONLINE)
	await assert_signal(emitter).is_emitted("online")
	nm.set_state(NMScript.NetState.OFFLINE)
	await assert_signal(emitter).is_emitted("offline")
	nm.set_state(NMScript.NetState.WEAK)
	await assert_signal(emitter).is_emitted("weak_network")
