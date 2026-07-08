extends GdUnitTestSuite

# SceneManager 测试: 初始状态 + pop guard + 无效路径错误守卫。
# 注: push/replace/reset_to 内部调用 get_tree().change_scene_to_file(), 会真实切换
# 测试树当前场景, 故 happy-path 栈逻辑无法在此安全测试(会破坏测试运行)。
# 无效路径会失败并早返回(不更新栈), 可安全验证守卫。建议后续重构把栈逻辑与
# 场景切换分离以获得完整可测性。

const SM_SCRIPT := "res://scripts/autoload/scene_manager.gd"
const BAD_SCENE := "res://tests/fixtures/does_not_exist.tscn"


func _new_sm() -> Node:
	var sm: Node = auto_free(load(SM_SCRIPT).new())
	add_child(sm)
	return sm


func test_initial_state() -> void:
	var sm := _new_sm()
	assert_int(sm.stack_size()).is_equal(0)
	assert_str(sm.current_scene_path()).is_equal("")


func test_pop_on_empty_returns_false() -> void:
	var sm := _new_sm()
	assert_bool(sm.pop()).is_false()
	assert_int(sm.stack_size()).is_equal(0)


func test_push_invalid_path_does_not_update_stack() -> void:
	var sm := _new_sm()
	sm.push(BAD_SCENE)  # change_scene_to_file 失败, 早返回, 栈不变
	assert_int(sm.stack_size()).is_equal(0)
	assert_str(sm.current_scene_path()).is_equal("")


func test_replace_invalid_path_does_not_update_stack() -> void:
	var sm := _new_sm()
	sm.replace(BAD_SCENE)
	assert_int(sm.stack_size()).is_equal(0)


func test_reset_to_invalid_path_does_not_update_stack() -> void:
	var sm := _new_sm()
	sm.reset_to(BAD_SCENE)
	assert_int(sm.stack_size()).is_equal(0)
