extends GdUnitTestSuite

# Logger 测试: _apply_level / _format / ring buffer / flush_crash 落盘。
# 用独立实例(不入树)避免 _ready 打开日志文件; 私有成员按 GDScript 约定可直接访问。
# Godot 4.7: autoload 全局名解析为类而非实例, 故 enum/const 走 preload 的脚本。

const LoggerScript := preload("res://scripts/core/logger.gd")
const LOG_DIR := "user://logs"
const LOG_FILE := "user://logs/app.log"
const CRASH_FILE := "user://logs/crash_last.log"


func before_test() -> void:
	_cleanup_logs()


func after_test() -> void:
	_cleanup_logs()


func _cleanup_logs() -> void:
	# 仅清理本测试创建的 crash_last.log; 不删 app.log(那是 autoload Logger 的文件,
	# 删除会使其 _file 句柄指向已删除 inode, 破坏后续 config_logger_integration 测试)
	if FileAccess.file_exists(CRASH_FILE):
		DirAccess.remove_absolute(CRASH_FILE)


func _new_logger() -> Variant:
	return auto_free(LoggerScript.new())


func test_apply_level() -> void:
	var l: Variant = _new_logger()
	l._apply_level("DEBUG")
	assert_int(l._min_level).is_equal(LoggerScript.Level.DEBUG)
	l._apply_level("INFO")
	assert_int(l._min_level).is_equal(LoggerScript.Level.INFO)
	l._apply_level("WARN")
	assert_int(l._min_level).is_equal(LoggerScript.Level.WARN)
	l._apply_level("WARNING")
	assert_int(l._min_level).is_equal(LoggerScript.Level.WARN)
	l._apply_level("ERROR")
	assert_int(l._min_level).is_equal(LoggerScript.Level.ERROR)
	l._apply_level("UNKNOWN")
	assert_int(l._min_level).is_equal(LoggerScript.Level.INFO)  # 未知 → INFO


func test_format_with_category() -> void:
	var l: Variant = _new_logger()
	var s: String = l._format(LoggerScript.Level.INFO, "Cat", "hello")
	assert_str(s).contains("INFO")
	assert_str(s).contains("[Cat]")
	assert_str(s).contains("hello")


func test_format_without_category() -> void:
	var l: Variant = _new_logger()
	var s: String = l._format(LoggerScript.Level.ERROR, "", "boom")
	assert_str(s).contains("ERROR")
	assert_str(s).contains("boom")


func test_ring_buffer_caps_at_max() -> void:
	var l: Variant = _new_logger()
	# _min_level 默认 INFO, _log(INFO) 会写入 _ring(_file 为 null, _append_file 静默跳过)
	for i in 600:
		l._log(LoggerScript.Level.INFO, "", "msg %d" % i)
	assert_int(l._ring.size()).is_equal(LoggerScript.RING_MAX)
	# 最新条目仍在, 最早的已被弹出
	assert_str(l._ring[l._ring.size() - 1]).contains("msg 599")


func test_level_filter_skips_below_min() -> void:
	var l: Variant = _new_logger()
	l._apply_level("ERROR")
	l._log(LoggerScript.Level.DEBUG, "", "should be filtered")
	l._log(LoggerScript.Level.INFO, "", "should be filtered")
	l._log(LoggerScript.Level.ERROR, "", "should appear")
	assert_int(l._ring.size()).is_equal(1)
	assert_str(l._ring[0]).contains("should appear")


func test_flush_crash_writes_file() -> void:
	var l: Variant = _new_logger()
	l._log(LoggerScript.Level.ERROR, "TestCat", "a crash entry")
	l.flush_crash("boom_reason")
	assert_bool(FileAccess.file_exists(CRASH_FILE)).is_true()
	var f := FileAccess.open(CRASH_FILE, FileAccess.READ)
	var content: String = f.get_as_text()
	f.close()
	assert_str(content).contains("boom_reason")
	assert_str(content).contains("a crash entry")


func test_report_crash_writes_snapshot() -> void:
	var l: Variant = _new_logger()
	l.info("before crash")
	l.report_crash("fatal_error", "detail info")
	assert_bool(FileAccess.file_exists(CRASH_FILE)).is_true()
	var f := FileAccess.open(CRASH_FILE, FileAccess.READ)
	var content: String = f.get_as_text()
	f.close()
	assert_str(content).contains("fatal_error")
	assert_str(content).contains("before crash")
