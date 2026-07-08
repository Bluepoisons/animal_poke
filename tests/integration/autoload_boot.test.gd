extends GdUnitTestSuite

# 集成测试: 验证 7 个 autoload 全部在场景树中注册并可访问。
# Godot 4.7: autoload 全局名解析为类, 故 enum 走 preload 的脚本, 实例走 /root/<Name>。
# project.godot [autoload] 顺序: ConfigManager → Logger → NetworkManager → SaveManager →
# AudioManager → SceneManager → GameManager。

const NMScript := preload("res://scripts/autoload/network_manager.gd")
const GMScript := preload("res://scripts/autoload/game_manager.gd")

const AUTOLOADS := ["ConfigManager", "Logger", "NetworkManager", "SaveManager", "AudioManager", "SceneManager", "GameManager"]


func test_all_autoloads_present() -> void:
	for n in AUTOLOADS:
		var node := get_node_or_null("/root/" + n)
		assert_bool(node != null).is_true()


func test_autoloads_are_nodes() -> void:
	for n in AUTOLOADS:
		var node := get_node_or_null("/root/" + n)
		if node != null:
			assert_bool(node is Node).is_true()


func test_game_manager_auto_transitions_to_main_menu() -> void:
	# GameManager._ready 应已自动转到 MAIN_MENU
	var gm := get_node_or_null("/root/GameManager")
	if gm == null:
		fail("GameManager autoload 不可用")
		return
	var state: int = gm.call("current_state")
	assert_int(state).is_equal(GMScript.GameState.MAIN_MENU)


func test_offline_guard_blocks_discover() -> void:
	# 集成: NetworkManager 设 OFFLINE 后, GameManager 转 DISCOVER 应被拦截
	var nm := get_node_or_null("/root/NetworkManager")
	var gm := get_node_or_null("/root/GameManager")
	if nm == null or gm == null:
		fail("autoload 不可用")
		return
	var prev: int = nm.call("current_state")
	nm.call("set_state", NMScript.NetState.OFFLINE)
	# 先确保不在 DISCOVER/CAPTURE(回到 MAIN_MENU)
	gm.call("transition_to", GMScript.GameState.MAIN_MENU)
	assert_bool(gm.call("transition_to", GMScript.GameState.DISCOVER)).is_false()
	# 恢复 ONLINE 后可进入
	nm.call("set_state", NMScript.NetState.ONLINE)
	assert_bool(gm.call("transition_to", GMScript.GameState.DISCOVER)).is_true()
	# 还原 NetworkManager 状态
	nm.call("set_state", prev)
