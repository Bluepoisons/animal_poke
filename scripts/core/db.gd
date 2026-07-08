class_name LocalDB
extends RefCounted

## LocalDB — 本地数据库层(F4): SQLite + 加密
## 封装 godot-sqlite 插件(https://github.com/2shady4u/godot-sqlite)的 CRUD。
## 表: animals / player_progress / inventory / checkin; schema 见 scripts/core/schema.sql。
##
## 安装插件: 将 godot-sqlite 的 addons/ 放入本项目 addons/ 并重启编辑器启用(插件管理器中勾选)。
##   未安装时 open() 会 push_error 并返回 false, 业务层(SaveManager)据此降级提示。
##
## 加密: 使用启用了 SQLCipher 的 godot-sqlite 构建时可设置 encryption_key 做本地静态加密
##   (防设备越权读取, 非密钥安全)。密钥由客户端本地生成并持久化于 user://.dbkey, 不进入 .env。

const SCHEMA_PATH := "res://scripts/core/schema.sql"
const ENCRYPTION_KEY_PATH := "user://.dbkey"

var _db = null          # SQLite 实例
var _opened := false


## 打开数据库并执行 schema(幂等 CREATE TABLE IF NOT EXISTS)。
func open(filename: String = "animal_poke.db") -> bool:
	if not ClassDB.class_exists("SQLite"):
		push_error("[LocalDB] 未找到 SQLite 类, 请先安装 godot-sqlite 插件(见文件头注释)。")
		return false
	_db = ClassDB.instantiate("SQLite")
	_db.path = "user://" + filename
	_db.encryption_key = _load_or_create_key()
	if not _db.open_db():
		push_error("[LocalDB] 打开数据库失败: %s" % _db.path)
		return false
	_opened = true
	_run_schema()
	return true


func close() -> void:
	if _db != null and _opened:
		_db.close_db()
	_opened = false


func is_open() -> bool:
	return _opened


func _run_schema() -> void:
	var sql := _read_file(SCHEMA_PATH)
	if sql == "":
		push_error("[LocalDB] 无法读取 schema: %s" % SCHEMA_PATH)
		return
	# 插件 query() 一次执行单条语句, 按 ";" 拆分
	for stmt in sql.split(";", false):
		var s := stmt.strip_edges()
		if s == "":
			continue
		if not _db.query(s):
			push_error("[LocalDB] 执行 schema 失败: %s | %s" % [s, _db.error_message])


# ---------- 动物表 CRUD ----------

## 写入或更新一只动物(以 uuid 为主键)。data 字段见 schema.sql(animals)。
func upsert_animal(data: Dictionary) -> bool:
	if not _ensure():
		return false
	var uuid := _sql_escape(str(data.get("uuid", "")))
	if _db.select_rows("animals", "uuid = '%s'" % uuid, []).size() > 0:
		return _db.update_rows("animals", data, "uuid = '%s'" % uuid, [])
	return _db.insert_rows("animals", data)


func get_animal(uuid: String) -> Dictionary:
	if not _ensure():
		return {}
	var res := _db.select_rows("animals", "uuid = '%s'" % _sql_escape(uuid), [])
	return res[0] if res.size() > 0 else {}


func get_all_animals() -> Array:
	if not _ensure():
		return []
	return _db.select_rows("animals", "", [])


func delete_animal(uuid: String) -> bool:
	if not _ensure():
		return false
	return _db.delete_rows("animals", "uuid = '%s'" % _sql_escape(uuid), [])


# ---------- 玩家进度 CRUD(单行, id=1) ----------

func upsert_progress(data: Dictionary) -> bool:
	if not _ensure():
		return false
	if _db.select_rows("player_progress", "id = 1", []).size() > 0:
		return _db.update_rows("player_progress", data, "id = 1", [])
	var row := data.duplicate()
	row["id"] = 1
	return _db.insert_rows("player_progress", row)


func get_progress() -> Dictionary:
	if not _ensure():
		return {}
	var res := _db.select_rows("player_progress", "id = 1", [])
	return res[0] if res.size() > 0 else {}


# ---------- 背包 CRUD ----------

func set_inventory(item_type: String, item_id: String, quantity: int) -> bool:
	if not _ensure():
		return false
	var cond := "item_type = '%s' AND item_id = '%s'" % [_sql_escape(item_type), _sql_escape(item_id)]
	if _db.select_rows("inventory", cond, []).size() > 0:
		return _db.update_rows("inventory", {"quantity": quantity}, cond, [])
	return _db.insert_rows("inventory", {"item_type": item_type, "item_id": item_id, "quantity": quantity})


func get_inventory() -> Array:
	if not _ensure():
		return []
	return _db.select_rows("inventory", "", [])


# ---------- 签到 CRUD ----------

func add_checkin(data: Dictionary) -> bool:
	if not _ensure():
		return false
	return _db.insert_rows("checkin", data)


func get_checkins() -> Array:
	if not _ensure():
		return []
	return _db.select_rows("checkin", "", [])


# ---------- 内部工具 ----------

func _ensure() -> bool:
	if not _opened:
		push_error("[LocalDB] 数据库未打开, 请先调用 open()。")
	return _opened


func _read_file(path: String) -> String:
	if not FileAccess.file_exists(path):
		return ""
	var f := FileAccess.open(path, FileAccess.READ)
	if f == null:
		return ""
	var s := f.get_as_text()
	f.close()
	return s


func _sql_escape(s: String) -> String:
	return s.replace("'", "''")


## 本地静态加密密钥: 首次生成并持久化于 user://.dbkey。
func _load_or_create_key() -> String:
	if FileAccess.file_exists(ENCRYPTION_KEY_PATH):
		var f := FileAccess.open(ENCRYPTION_KEY_PATH, FileAccess.READ)
		if f != null:
			var k := f.get_line().strip_edges()
			f.close()
			if k != "":
				return k
	var key := _random_key()
	var f := FileAccess.open(ENCRYPTION_KEY_PATH, FileAccess.WRITE)
	if f != null:
		f.store_string(key)
		f.close()
	return key


func _random_key() -> String:
	var s := ""
	for i in 32:
		s += "%02x" % (randi() % 256)
	return s
