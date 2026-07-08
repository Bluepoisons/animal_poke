class_name ConfigLoader
extends RefCounted

## ConfigLoader — 客户端配置读取层(F3)
## 健壮解析 .env(支持注释 #、键值、成对引号、类型转换)。读取优先级:
##   OS 环境变量  >  .env 文件缓存  >  默认值
##
## 设计原则(见 4.6.1 Key 管理规范): 客户端 .env 只含非敏感配置
## (BACKEND_BASE_URL 等)。任何第三方 Key(腾讯地图/彩云/VLM/LLM)严禁出现在
## 客户端, 全部存放在 Go 后端 .env(见 F6)。
##
## 用法:
##   var cfg := ConfigLoader.new()
##   cfg.load_file("res://.env")
##   var url := cfg.get_string("BACKEND_BASE_URL", "http://127.0.0.1:8080")

const ENV_PATH := "res://.env"

var _cache: Dictionary = {}


func _init() -> void:
	load_file(ENV_PATH)


## 解析 .env 文件。文件不存在时静默返回(配置可来自 OS 环境变量 / 默认值)。
func load_file(path: String) -> void:
	if not FileAccess.file_exists(path):
		return
	var f := FileAccess.open(path, FileAccess.READ)
	if f == null:
		push_warning("[ConfigLoader] 无法打开 %s: %s" % [path, FileAccess.get_open_error()])
		return
	while not f.eof_reached():
		var line := f.get_line().strip_edges()
		if line == "" or line.begins_with("#"):
			continue
		var eq := line.find("=")
		if eq < 0:
			continue
		var k := line.substr(0, eq).strip_edges()
		var v := line.substr(eq + 1).strip_edges()
		# 去掉成对的单/双引号
		if v.length() >= 2:
			if (v.begins_with("\"") and v.ends_with("\"")) or (v.begins_with("'") and v.ends_with("'")):
				v = v.substr(1, v.length() - 2)
		if k != "":
			_cache[k] = v
	f.close()


## 原始字符串值: OS 环境变量优先, 其次 .env 缓存, 最后 default。
func get_raw(key: String, default: String = "") -> String:
	var os_val := OS.get_environment(key)
	if os_val != "":
		return os_val
	if _cache.has(key):
		return _cache[key]
	return default


func get_string(key: String, default: String = "") -> String:
	return get_raw(key, default)


func get_int(key: String, default: int = 0) -> int:
	var s := get_raw(key, "")
	if s == "":
		return default
	return s.to_int()


func get_float(key: String, default: float = 0.0) -> float:
	var s := get_raw(key, "")
	if s == "":
		return default
	return s.to_float()


func get_bool(key: String, default: bool = false) -> bool:
	var s := get_raw(key, "").to_lower()
	if s == "":
		return default
	return s == "true" or s == "1" or s == "yes"


## 是否配置了某 key(非空)
func has(key: String) -> bool:
	return get_raw(key, "") != ""
