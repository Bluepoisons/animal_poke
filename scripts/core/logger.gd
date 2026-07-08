extends Node

## Logger — 分级日志 + 崩溃上报骨架(F5)
## 级别: DEBUG < INFO < WARN < ERROR。每条日志同时写入:
##   1) 内存环形缓冲(最近 RING_MAX 条, report_crash() 时落盘)
##   2) 本地文件 user://logs/app.log(实时追加, 硬崩溃前已落盘)
## 崩溃落盘: 正常退出仅 flush 文件缓冲; report_crash() 显式触发时将环形缓冲写入
##   user://logs/crash_last.log。硬崩溃(段错误/kill)GDScript 无法捕获, 后续接
##   Bugly/自建时在 flush_crash() 内上报 _ring(F5"后续接 Bugly/自建")。
## 参考: 九-风险(崩溃率<0.5% 验收, 需日志支撑)。
##
## 使用: Logger.debug("msg", "Category") / Logger.info(...) / Logger.warn(...) / Logger.error(...)
## 日志级别可由 ConfigManager.get_log_level() 注入(默认 INFO)。

enum Level { DEBUG, INFO, WARN, ERROR }

const LOG_DIR := "user://logs"
const LOG_FILE := "user://logs/app.log"
const CRASH_FILE := "user://logs/crash_last.log"
const RING_MAX := 500

var _min_level: int = Level.INFO
var _ring: Array[String] = []
var _file: FileAccess = null


func _ready() -> void:
	# 日志级别由 ConfigManager 注入(运行时查找, 规避 autoload 符号时序)
	var cm := get_node_or_null("/root/ConfigManager")
	if cm != null and cm.has_method("get_log_level"):
		_apply_level(cm.call("get_log_level"))
	_ensure_dir(LOG_DIR)
	_open_file()
	_log(Level.INFO, "Logger", "日志系统初始化完成 (level=%s)" % Level.keys()[_min_level])


func _notification(what: int) -> void:
	if what == NOTIFICATION_WM_CLOSE_REQUEST or what == NOTIFICATION_PREDELETE:
		# 正常退出仅刷新文件缓冲; crash_last.log 由 report_crash() 显式触发,
		# 避免每次退出覆盖真实崩溃快照。硬崩溃(段错误/kill)GDScript 无法捕获,
		# 需后续接 OS 级信号处理器(F5"后续接 Bugly/自建"), flush_crash() 已预留入口。
		flush()


func _apply_level(s: String) -> void:
	match s.to_upper():
		"DEBUG": _min_level = Level.DEBUG
		"INFO": _min_level = Level.INFO
		"WARN", "WARNING": _min_level = Level.WARN
		"ERROR": _min_level = Level.ERROR
		_: _min_level = Level.INFO


func debug(msg: String, category: String = "") -> void: _log(Level.DEBUG, category, msg)
func info(msg: String, category: String = "") -> void: _log(Level.INFO, category, msg)
func warn(msg: String, category: String = "") -> void: _log(Level.WARN, category, msg)
func error(msg: String, category: String = "") -> void: _log(Level.ERROR, category, msg)


func _log(level: int, category: String, msg: String) -> void:
	if level < _min_level:
		return
	var entry := _format(level, category, msg)
	_ring.append(entry)
	if _ring.size() > RING_MAX:
		_ring.pop_front()
	# 实时镜像到 Godot 输出(ERROR/WARN 走 push_* 以便被编辑器错误面板捕获)
	if level >= Level.ERROR:
		push_error(entry)
	elif level >= Level.WARN:
		push_warning(entry)
	else:
		print(entry)
	_append_file(entry)


func _format(level: int, category: String, msg: String) -> String:
	var ts := Time.get_datetime_string_from_system(true)
	var lvl: String = Level.keys()[level]
	if category != "":
		return "%s [%s] [%s] %s" % [ts, lvl, category, msg]
	return "%s [%s] %s" % [ts, lvl, msg]


func _ensure_dir(path: String) -> void:
	if not DirAccess.dir_exists_absolute(path):
		DirAccess.make_dir_recursive_absolute(path)


func _open_file() -> void:
	# Godot 4.7: WRITE_READ 不再创建新文件, 故按存在性选择模式。
	# 已存在 → READ_WRITE(不截断, 配合 seek_end 追加); 不存在 → WRITE(创建)。
	if FileAccess.file_exists(LOG_FILE):
		_file = FileAccess.open(LOG_FILE, FileAccess.READ_WRITE)
	else:
		_file = FileAccess.open(LOG_FILE, FileAccess.WRITE)
	if _file == null:
		push_warning("[Logger] 无法打开日志文件 %s" % FileAccess.get_open_error())
		return
	_file.seek_end(0)


func _append_file(entry: String) -> void:
	if _file == null:
		return
	_file.store_line(entry)
	_file.flush()


## 显式触发崩溃上报(业务捕获到致命错误时调用)。
func report_crash(reason: String, detail: String = "") -> void:
	if detail != "":
		_ring.append("[CRASH] %s | %s" % [reason, detail])
	flush_crash(reason)


## 将内存环形缓冲落盘为崩溃快照(同时可作为 Bugly/自建上报的入口)。
func flush_crash(reason: String) -> void:
	_ensure_dir(LOG_DIR)
	var f := FileAccess.open(CRASH_FILE, FileAccess.WRITE)
	if f == null:
		return
	f.store_line("=== crash snapshot @ %s ===" % Time.get_datetime_string_from_system(true))
	f.store_line("reason: %s" % reason)
	for e in _ring:
		f.store_line(e)
	f.close()
	# TODO(后续): 在此调用 Bugly / 自建上报接口, 上报 _ring 内容。


## 手动刷新文件缓冲(正常退出或定期调用)。
func flush() -> void:
	if _file != null:
		_file.flush()
