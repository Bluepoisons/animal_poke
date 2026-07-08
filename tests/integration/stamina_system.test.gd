extends GdUnitTestSuite

# 集成测试 — StaminaManager 持久化与 autoload 链路 (M1-MVP, issue #1)
#
# 场景:
#   1. DB 层: 体力数据写入 LocalDB → 重启(新实例打开同库) → 数据仍在
#   2. Autoload 链路: StaminaManager → SaveManager → LocalDB 读写正常
#
# 用独立临时 db 文件, 不污染真实存档(animal_poke.db)。不删 user://.dbkey。

const TMP_DB := "test_integration_stamina.db"
const TMP_DB_PATH := "user://test_integration_stamina.db"

const RECOVERY_INTERVAL_SEC: int = 360
const BASE_MAX: int = 120

const STM_SCRIPT := preload("res://scripts/modules/stamina/stamina_manager.gd")


func before_test() -> void:
	_cleanup()


func after_test() -> void:
	_cleanup()


func _cleanup() -> void:
	if FileAccess.file_exists(TMP_DB_PATH):
		DirAccess.remove_absolute(TMP_DB_PATH)


func _open_db() -> LocalDB:
	var db := LocalDB.new()
	assert_bool(db.open(TMP_DB)).is_true()
	return db


# ---------- 辅助: 插入完整 progress 行 ----------

func _insert_progress(db: LocalDB, stamina: int, stamina_updated_at: String, level: int = 1) -> void:
	assert_bool(db.upsert_progress({
		"level": level,
		"exp": 0,
		"coins": 0,
		"stamina": stamina,
		"stamina_updated_at": stamina_updated_at,
		"total_captured": 0,
	})).is_true()


# ---------- DB 层: 体力数据重启不丢 ----------

func test_stamina_restart_persistence() -> void:
	if not ClassDB.class_exists("SQLite"):
		print("[stamina_system] godot-sqlite 未加载, 跳过")
		return

	# --- 会话 1: 写入体力数据 ---
	var db1 := _open_db()
	var past := str(int(Time.get_unix_time_from_system()) - 720)
	_insert_progress(db1, 80, past, 2)

	# 快照
	var prog1 := db1.get_progress()
	assert_int(prog1["stamina"]).is_equal(80)
	assert_int(prog1["level"]).is_equal(2)

	db1.close()

	# --- 会话 2: 新实例打开同库 ---
	var db2 := _open_db()
	var prog2 := db2.get_progress()

	assert_int(prog2["stamina"]).is_equal(80)
	assert_int(prog2["level"]).is_equal(2)
	assert_str(prog2["stamina_updated_at"]).is_equal(past)

	db2.close()


func test_stamina_zero_value_persists() -> void:
	if not ClassDB.class_exists("SQLite"):
		return

	var db1 := _open_db()
	_insert_progress(db1, 0, str(int(Time.get_unix_time_from_system())))

	var prog := db1.get_progress()
	assert_int(prog["stamina"]).is_equal(0)

	db1.close()

	var db2 := _open_db()
	assert_int(db2.get_progress()["stamina"]).is_equal(0)
	db2.close()


# ---------- Autoload 链路: StaminaManager 正确注册 ----------

func test_stamina_manager_autoload_present() -> void:
	var stm := get_node_or_null("/root/StaminaManager")
	if stm == null:
		fail("StaminaManager autoload 未注册, 请检查 project.godot")
		return
	assert_object(stm).is_not_null()


func test_stamina_manager_has_expected_methods() -> void:
	if not ClassDB.class_exists("SQLite"):
		return

	var stm := get_node_or_null("/root/StaminaManager")
	if stm == null:
		return

	assert_bool(stm.has_method("can_capture")).is_true()
	assert_bool(stm.has_method("consume_for_capture")).is_true()
	assert_bool(stm.has_method("restore_to_full")).is_true()
	assert_bool(stm.has_method("on_level_up")).is_true()
	assert_bool(stm.has_method("purchase_stamina")).is_true()
	assert_bool(stm.has_method("get_current")).is_true()
	assert_bool(stm.has_method("get_max")).is_true()


# ---------- SaveManager → StaminaManager 读链路 ----------

func test_stamina_manager_reads_from_savemanager() -> void:
	if not ClassDB.class_exists("SQLite"):
		return

	var sm := get_node_or_null("/root/SaveManager")
	var stm := get_node_or_null("/root/StaminaManager")
	if sm == null or stm == null:
		fail("autoload 不可用")
		return

	# 保存原始数据以便恢复
	var original: Dictionary = sm.call("load_progress")

	# 写入测试数据
	sm.call("save_progress", {
		"level": 2,
		"exp": 0,
		"coins": 0,
		"stamina": 50,
		"stamina_updated_at": str(int(Time.get_unix_time_from_system())),
		"total_captured": 0,
	})

	# 触发 StaminaManager 重新加载
	stm.call("_load_and_recover")

	# 验证加载正确
	assert_int(stm.get("_current_stamina")).is_equal(50)
	assert_int(stm.get("_max_stamina")).is_equal(134)  # Lv.2 = 120 + 14

	# 恢复原始数据
	if not original.is_empty():
		sm.call("save_progress", original)


# ---------- StaminaManager → SaveManager 写链路 ----------

func test_consume_writes_to_savemanager() -> void:
	if not ClassDB.class_exists("SQLite"):
		return

	var sm := get_node_or_null("/root/SaveManager")
	var stm := get_node_or_null("/root/StaminaManager")
	if sm == null or stm == null:
		fail("autoload 不可用")
		return

	var original: Dictionary = sm.call("load_progress")

	# 设置初始体力
	stm.set("_current_stamina", 80)
	stm.set("_last_update_unix", int(Time.get_unix_time_from_system()))

	# 消耗
	assert_bool(stm.call("consume_for_capture")).is_true()

	# 验证 DB 已更新
	var saved: Dictionary = sm.call("load_progress")
	assert_int(saved["stamina"]).is_equal(60)

	if not original.is_empty():
		sm.call("save_progress", original)


# ---------- 离线恢复写入 ----------

func test_recover_updates_savemanager_timestamp() -> void:
	if not ClassDB.class_exists("SQLite"):
		return

	var sm := get_node_or_null("/root/SaveManager")
	var stm := get_node_or_null("/root/StaminaManager")
	if sm == null or stm == null:
		fail("autoload 不可用")
		return

	var original: Dictionary = sm.call("load_progress")

	# 设置过去的时间戳
	var past := int(Time.get_unix_time_from_system()) - RECOVERY_INTERVAL_SEC * 3
	stm.set("_last_update_unix", past)
	stm.set("_current_stamina", 80)
	stm.call("_recover")

	# 验证 DB 中 timestamp 已更新
	var saved: Dictionary = sm.call("load_progress")
	var saved_ts := int(saved.get("stamina_updated_at", "0"))
	assert_bool(saved_ts > past).is_true()

	if not original.is_empty():
		sm.call("save_progress", original)


# ---------- 空数据库首次初始化 ----------

func test_first_time_initialization_with_temp_db() -> void:
	if not ClassDB.class_exists("SQLite"):
		return

	# 使用空数据库, 验证默认值
	var db := _open_db()
	var prog := db.get_progress()

	# 新库无数据
	assert_dict(prog).is_empty()

	# 插入初始数据
	_insert_progress(db, BASE_MAX, str(int(Time.get_unix_time_from_system())), 1)

	prog = db.get_progress()
	assert_int(prog["stamina"]).is_equal(BASE_MAX)
	assert_int(prog["level"]).is_equal(1)

	db.close()
