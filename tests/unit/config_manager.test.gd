extends GdUnitTestSuite

# ConfigManager 测试: get_endpoint 路径拼接 + 设备 Token 持久化。
# 注: GDScript 无法设置 OS 环境变量, .env 写入会污染项目, 故 base URL 的具体值
# 不做硬断言, 改测路径拼接逻辑(与 base 无关)。设备 Token 完全可测(user:// 可控)。

const CM_SCRIPT := "res://scripts/autoload/config_manager.gd"
const TOKEN_PATH := "user://auth/device_token.txt"


func before_test() -> void:
	_cleanup_token()


func after_test() -> void:
	_cleanup_token()


func _cleanup_token() -> void:
	if FileAccess.file_exists(TOKEN_PATH):
		DirAccess.remove_absolute(TOKEN_PATH)


# ConfigManager._ready 创建 _loader, 必须入树触发。auto_free 负责回收。
func _new_cm() -> Node:
	var cm: Node = auto_free(load(CM_SCRIPT).new())
	add_child(cm)
	return cm


func test_get_endpoint_with_leading_slash() -> void:
	var cm := _new_cm()
	var ep: String = cm.get_endpoint("/auth/device")
	# 与 base 值无关, 验证拼接: 末尾必须是 /auth/device
	assert_str(ep).ends_with("/auth/device")
	# 含协议头
	assert_bool(ep.contains("://")).is_true()
	# 不应出现双斜杠(协议:// 之后)
	assert_bool(ep.contains("//auth")).is_false()


func test_get_endpoint_without_leading_slash() -> void:
	var cm := _new_cm()
	# 无前导 / 应自动补
	var with_slash: String = cm.get_endpoint("/auth/device")
	var without_slash: String = cm.get_endpoint("auth/device")
	assert_str(without_slash).is_equal(with_slash)


func test_get_endpoint_normalizes_trailing_slash_in_base() -> void:
	var cm := _new_cm()
	# get_endpoint 内部对 base 做 trim_suffix("/"), 故结果不应以 //auth 结尾
	var ep: String = cm.get_endpoint("/ping")
	assert_str(ep).ends_with("/ping")
	assert_bool(ep.contains("//ping")).is_false()


func test_device_token_round_trip() -> void:
	var cm := _new_cm()
	assert_str(cm.get_device_token()).is_equal("")
	cm.set_device_token("abc123")
	assert_str(cm.get_device_token()).is_equal("abc123")
	cm.clear_device_token()
	assert_str(cm.get_device_token()).is_equal("")


func test_set_device_token_creates_directory() -> void:
	var cm := _new_cm()
	_cleanup_token()
	# user://auth 不存在时, set 应自动创建目录并写入
	cm.set_device_token("xyz")
	assert_bool(FileAccess.file_exists(TOKEN_PATH)).is_true()
	# 再次 set(覆盖)
	cm.set_device_token("new_value")
	assert_str(cm.get_device_token()).is_equal("new_value")


func test_config_getters_non_empty() -> void:
	# 不硬断言具体值(.env 可能覆盖), 只验证返回有效字符串
	var cm := _new_cm()
	assert_bool(cm.get_db_filename().length() > 0).is_true()
	assert_bool(cm.get_log_level().length() > 0).is_true()
	assert_bool(cm.get_backend_base_url().length() > 0).is_true()
