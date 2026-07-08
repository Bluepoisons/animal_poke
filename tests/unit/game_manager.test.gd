extends GdUnitTestSuite

# GameManager 测试: 状态机逻辑。用独立实例(不入树)避免 _ready 自动跳转 MAIN_MENU,
# 且 _is_transition_valid 的 NetworkManager 查找返回 null → 不触发离线拦截(离线拦截测集成)。
# Godot 4.7: enum 走 preload 的脚本, 实例走 Variant 动态分发。

const GMScript := preload("res://scripts/autoload/game_manager.gd")


func _new_gm() -> Variant:
	return auto_free(GMScript.new())


func test_initial_state() -> void:
	var gm: Variant = _new_gm()
	assert_int(gm.current_state()).is_equal(GMScript.GameState.BOOT)
	assert_int(gm.previous_state()).is_equal(GMScript.GameState.BOOT)


func test_state_name() -> void:
	var gm: Variant = _new_gm()
	assert_str(gm.state_name(GMScript.GameState.BOOT)).is_equal("BOOT")
	assert_str(gm.state_name(GMScript.GameState.MAIN_MENU)).is_equal("MAIN_MENU")
	assert_str(gm.state_name(GMScript.GameState.DISCOVER)).is_equal("DISCOVER")
	assert_str(gm.state_name(GMScript.GameState.CAPTURE)).is_equal("CAPTURE")
	assert_str(gm.state_name(GMScript.GameState.COLLECT)).is_equal("COLLECT")
	assert_str(gm.state_name(GMScript.GameState.BATTLE)).is_equal("BATTLE")
	assert_str(gm.state_name(GMScript.GameState.SETTINGS)).is_equal("SETTINGS")


func test_transition_same_state_is_noop() -> void:
	var gm: Variant = _new_gm()
	var emitter := monitor_signals(gm)
	assert_bool(gm.transition_to(GMScript.GameState.BOOT)).is_true()
	assert_int(gm.current_state()).is_equal(GMScript.GameState.BOOT)
	assert_int(gm.previous_state()).is_equal(GMScript.GameState.BOOT)
	await await_millis(30)
	assert_signal(emitter).is_not_emitted("state_changed")


func test_transition_boot_to_main_menu() -> void:
	var gm: Variant = _new_gm()
	var emitter := monitor_signals(gm)
	assert_bool(gm.transition_to(GMScript.GameState.MAIN_MENU)).is_true()
	assert_int(gm.current_state()).is_equal(GMScript.GameState.MAIN_MENU)
	assert_int(gm.previous_state()).is_equal(GMScript.GameState.BOOT)
	await assert_signal(emitter).is_emitted("state_changed", [GMScript.GameState.BOOT, GMScript.GameState.MAIN_MENU])


func test_transition_chain_tracks_previous() -> void:
	var gm: Variant = _new_gm()
	# BOOT -> MAIN_MENU -> DISCOVER -> CAPTURE -> COLLECT
	assert_bool(gm.transition_to(GMScript.GameState.MAIN_MENU)).is_true()
	assert_int(gm.previous_state()).is_equal(GMScript.GameState.BOOT)

	assert_bool(gm.transition_to(GMScript.GameState.DISCOVER)).is_true()
	assert_int(gm.previous_state()).is_equal(GMScript.GameState.MAIN_MENU)
	assert_int(gm.current_state()).is_equal(GMScript.GameState.DISCOVER)

	assert_bool(gm.transition_to(GMScript.GameState.CAPTURE)).is_true()
	assert_int(gm.previous_state()).is_equal(GMScript.GameState.DISCOVER)

	assert_bool(gm.transition_to(GMScript.GameState.COLLECT)).is_true()
	assert_int(gm.previous_state()).is_equal(GMScript.GameState.CAPTURE)


func test_transition_to_discover_succeeds_without_network_guard() -> void:
	# 独立实例不入树, get_node_or_null("/root/NetworkManager") 返回 null,
	# 离线拦截不触发, DISCOVER 可进入(离线拦截的集成测试见 integration)
	var gm: Variant = _new_gm()
	assert_bool(gm.transition_to(GMScript.GameState.MAIN_MENU)).is_true()
	assert_bool(gm.transition_to(GMScript.GameState.DISCOVER)).is_true()
	assert_bool(gm.transition_to(GMScript.GameState.CAPTURE)).is_true()
