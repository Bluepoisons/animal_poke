extends Node

## ConfigManager — 配置读取(单例)
## 统一读取 .env 配置(API key / endpoint 等)。完整实现见 F3。
## 参考: 游戏开发计划.md 4.6 API Key 管理规范、项目约定(.env 已 gitignore)。
##
## 读取优先级: OS 环境变量 > .env 文件缓存 > 默认值
## 重要: 所有 API key 必须经此单例读取, 禁止硬编码进代码或提交 git。

const ENV_PATH := "res://.env"

# 已知配置键(F3 会补充完整字段表, 见 .env.example)
const KEY_TENCENT_MAP := "TENCENT_MAP_KEY"
const KEY_CAIYUN_WEATHER := "CAIYUN_WEATHER_KEY"
const KEY_VLM_ENDPOINT := "VLM_ENDPOINT"
const KEY_VLM_KEY := "VLM_KEY"
const KEY_LLM_ENDPOINT := "LLM_ENDPOINT"
const KEY_LLM_KEY := "LLM_KEY"

var _cache: Dictionary = {}


func _ready() -> void:
	_load_env_file()
	print("[ConfigManager] 初始化完成 (骨架, 完整实现见 F3)")


## 读取配置值。优先 OS 环境变量, 其次 .env 缓存, 最后 default。
func get_value(key: String, default: Variant = null) -> Variant:
	var os_val := OS.get_environment(key)
	if os_val != "":
		return os_val
	if _cache.has(key):
		return _cache[key]
	return default


## 是否已配置某 key(非空)
func has_key(key: String) -> bool:
	var v: Variant = get_value(key, "")
	return v is String and v != ""


## 解析 .env 文件到缓存。F3 会替换为更健壮的实现(支持注释/引号/多行/类型转换)。
func _load_env_file() -> void:
	if not FileAccess.file_exists(ENV_PATH):
		# .env 不存在属正常(开发期/CI), 配置可从 OS 环境变量读取
		return
	var f := FileAccess.open(ENV_PATH, FileAccess.READ)
	if f == null:
		push_warning("[ConfigManager] 无法打开 .env: %s" % FileAccess.get_open_error())
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
		# 去掉首尾配对引号
		if v.length() >= 2:
			if (v.begins_with("\"") and v.ends_with("\"")) or (v.begins_with("'") and v.ends_with("'")):
				v = v.substr(1, v.length() - 2)
		_cache[k] = v
	f.close()
