extends Node

## SaveManager — 本地存档管理(单例, 初始化顺序第 3)
## F4: 委托 LocalDB(SQLite + 加密)持久化。表: animals / player_progress / inventory / checkin。
## 参考: 4.4 同步方案、4.1 本地数据库、4.5 隐私(原始照片不落盘)。
##
## 重启不丢数据: 数据库存于 user://, 进程退出时 close(), 下次启动 open() 复用。

signal saved()
signal loaded()

var _db: LocalDB


func _ready() -> void:
	_db = LocalDB.new()
	# 数据库文件名取自 ConfigManager(默认 animal_poke.db)
	var fname := "animal_poke.db"
	var cm := get_node_or_null("/root/ConfigManager")
	if cm != null and cm.has_method("get_db_filename"):
		fname = cm.call("get_db_filename")
	if _db.open(fname):
		print("[SaveManager] 本地数据库就绪: %s" % fname)
	else:
		push_error("[SaveManager] 本地数据库初始化失败, 请安装 godot-sqlite 插件(F4)。")


func is_ready() -> bool:
	return _db != null and _db.is_open()


## 暴露内部 LocalDB 实例(可能为 null), 供业务模块(M8 AnimalRepository 等)做复杂查询/批量操作。
## 简单 CRUD 仍建议走 save_animal/load_animals 等高层方法。
func get_db() -> LocalDB:
	return _db


## 保存单只动物元数据(捕获后调用)。字段见 schema.sql(animals)。
func save_animal(data: Dictionary) -> bool:
	return _db.upsert_animal(data)


## 读取全部动物(图鉴浏览用, 离线可)
func load_animals() -> Array:
	return _db.get_all_animals()


## 保存玩家进度(等级/金币/体力等)
func save_progress(data: Dictionary) -> bool:
	return _db.upsert_progress(data)


func load_progress() -> Dictionary:
	return _db.get_progress()


## 设置背包物品数量
func set_inventory(item_type: String, item_id: String, quantity: int) -> bool:
	return _db.set_inventory(item_type, item_id, quantity)


func load_inventory() -> Array:
	return _db.get_inventory()


## 记录一次签到
func record_checkin(data: Dictionary) -> bool:
	return _db.add_checkin(data)


func load_checkins() -> Array:
	return _db.get_checkins()


func _notification(what: int) -> void:
	if what == NOTIFICATION_WM_CLOSE_REQUEST or what == NOTIFICATION_PREDELETE:
		if _db != null:
			_db.close()
