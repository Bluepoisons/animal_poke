extends Node

## ConfigManager — 客户端配置读取(单例, 初始化顺序第 1)
## F3 完善: 委托 ConfigLoader 读取非敏感客户端配置; 管理后端下发的设备 Token。
## 参考: 4.6.1 Key 管理规范、4.6.2 后端端点。
##
## 关键约束: 客户端 .env 只含 BACKEND_BASE_URL 等非敏感字段,
## 任何第三方 Key(腾讯地图/彩云/VLM/LLM)均在 Go 后端 .env, 客户端永不含。
##
## 历史: F1 阶段曾把第三方 Key 常量放此处; F3 起全部移除(F6 移到后端)。

const DEVICE_TOKEN_PATH := "user://auth/device_token.txt"

var _loader: ConfigLoader


func _ready() -> void:
	_loader = ConfigLoader.new()
	var has_url := _loader.has("BACKEND_BASE_URL")
	print("[ConfigManager] 初始化完成. BACKEND_BASE_URL=%s" % get_backend_base_url())
	if not has_url:
		push_warning("[ConfigManager] 未配置 BACKEND_BASE_URL, 使用默认值; 生产环境请在 .env 设置。")


## Go 后端基础地址(客户端唯一需直连的外部地址)。
func get_backend_base_url() -> String:
	return _loader.get_string("BACKEND_BASE_URL", "http://127.0.0.1:8080")


## 端点拼接: get_endpoint("/auth/device") -> http://host:port/auth/device
func get_endpoint(path: String) -> String:
	var base := get_backend_base_url().strip_edges().trim_suffix("/")
	if not path.begins_with("/"):
		path = "/" + path
	return base + path


## 日志级别(供 Logger 读取)
func get_log_level() -> String:
	return _loader.get_string("LOG_LEVEL", "INFO")


## 本地数据库文件名(相对 user://)
func get_db_filename() -> String:
	return _loader.get_string("DB_FILENAME", "animal_poke.db")


# ---- 设备 Token 管理(登录后由后端下发, 持久化于 user://, 不进 .env) ----

func get_device_token() -> String:
	if not FileAccess.file_exists(DEVICE_TOKEN_PATH):
		return ""
	var f := FileAccess.open(DEVICE_TOKEN_PATH, FileAccess.READ)
	if f == null:
		return ""
	var t := f.get_line().strip_edges()
	f.close()
	return t


func set_device_token(token: String) -> void:
	var dir := DEVICE_TOKEN_PATH.get_base_dir()
	if not DirAccess.dir_exists_absolute(dir):
		DirAccess.make_dir_recursive_absolute(dir)
	var f := FileAccess.open(DEVICE_TOKEN_PATH, FileAccess.WRITE)
	if f == null:
		push_error("[ConfigManager] 无法写入设备 Token: %s" % FileAccess.get_open_error())
		return
	f.store_string(token)
	f.close()


func clear_device_token() -> void:
	if FileAccess.file_exists(DEVICE_TOKEN_PATH):
		DirAccess.remove_absolute(DEVICE_TOKEN_PATH)
