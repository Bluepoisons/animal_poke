extends GdUnitTestSuite

# 集成测试: Logger 写 user://logs/app.log + 级别过滤。
# Godot 4.7: autoload 全局名解析为类, 故用 /root/Logger 取实例 + call()/get() 访问。
# Logger 是 autoload, _file 在 _ready 已打开(append), 故不删除 app.log(会导致 Logger
# 写入已删除 inode)。用唯一 marker 避免跨用例污染。修改 _min_level 后在 after 还原。

const LOG_FILE := "user://logs/app.log"

var _saved_min_level: int = 0


func before_test() -> void:
	_saved_min_level = get_logger().get("_min_level")


func after_test() -> void:
	get_logger().set("_min_level", _saved_min_level)


func get_logger() -> Node:
	var l := get_node_or_null("/root/Logger")
	assert_bool(l != null).is_true()
	return l


func _read_app_log() -> String:
	if not FileAccess.file_exists(LOG_FILE):
		return ""
	var f := FileAccess.open(LOG_FILE, FileAccess.READ)
	if f == null:
		return ""
	var s: String = f.get_as_text()
	f.close()
	return s


func test_error_log_persists_to_file() -> void:
	get_logger().call("error", "INTEGRATION_ERROR_MARKER_42")
	await await_millis(50)  # 让 store_line + flush 落盘
	assert_str(_read_app_log()).contains("INTEGRATION_ERROR_MARKER_42")


func test_level_filter_integration() -> void:
	var logger := get_logger()
	logger.call("_apply_level", "ERROR")
	logger.call("info", "FILTERED_INFO_MARKER_99")
	logger.call("error", "PASSED_ERROR_MARKER_99")
	await await_millis(50)
	var content := _read_app_log()
	assert_str(content).contains("PASSED_ERROR_MARKER_99")
	# INFO 在 ERROR 级别下被过滤, 不应落盘
	assert_bool(content.contains("FILTERED_INFO_MARKER_99")).is_false()


func test_warn_filtered_below_error_level() -> void:
	var logger := get_logger()
	logger.call("_apply_level", "ERROR")
	logger.call("warn", "FILTERED_WARN_MARKER_7")
	await await_millis(50)
	assert_bool(_read_app_log().contains("FILTERED_WARN_MARKER_7")).is_false()
