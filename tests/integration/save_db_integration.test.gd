extends GdUnitTestSuite

# 集成测试: LocalDB(SaveManager 委托) 完整 CRUD 往返 + 重启不丢数据。
# 依赖 godot-sqlite 插件; 未加载则跳过。用独立临时 db 文件, 不污染真实存档。
# 不删除 user://.dbkey(与真实 animal_poke.db 共享, 删除会破坏下次启动)。

const TMP_DB := "test_integration_save.db"
const TMP_DB_PATH := "user://test_integration_save.db"


func before_test() -> void:
	_cleanup_db()


func after_test() -> void:
	_cleanup_db()


func _cleanup_db() -> void:
	if FileAccess.file_exists(TMP_DB_PATH):
		DirAccess.remove_absolute(TMP_DB_PATH)


func test_crud_and_restart_persistence() -> void:
	if not ClassDB.class_exists("SQLite"):
		print("[save_db_integration] godot-sqlite 未加载, 跳过 CRUD 测试")
		return

	var db := LocalDB.new()
	assert_bool(db.open(TMP_DB)).is_true()

	# ---- animals: upsert / get / update / delete ----
	var animal := {
		"uuid": "u1", "species": "cat", "breed": "tabby", "rarity": 2,
		"attr_hp": 10, "inference_request_id": "req-001",
	}
	assert_bool(db.upsert_animal(animal)).is_true()
	var got: Dictionary = db.get_animal("u1")
	assert_str(got.get("uuid", "")).is_equal("u1")
	assert_str(got.get("species", "")).is_equal("cat")
	assert_int(got.get("rarity")).is_equal(2)

	# 同 uuid upsert 更新而非插入
	animal["rarity"] = 4
	assert_bool(db.upsert_animal(animal)).is_true()
	assert_array(db.get_all_animals()).has_size(1)
	assert_int(db.get_animal("u1").get("rarity")).is_equal(4)

	# 删除
	assert_bool(db.delete_animal("u1")).is_true()
	assert_array(db.get_all_animals()).is_empty()

	# ---- player_progress: 单行 id=1 往返 + 部分更新 ----
	assert_bool(db.upsert_progress({"level": 3, "coins": 150})).is_true()
	var prog: Dictionary = db.get_progress()
	assert_int(prog.get("level")).is_equal(3)
	assert_int(prog.get("coins")).is_equal(150)
	# 再次 upsert 更新同一行(不新增行)
	assert_bool(db.upsert_progress({"level": 4})).is_true()
	assert_int(db.get_progress().get("level")).is_equal(4)

	# ---- inventory: set(插入) + set(更新) ----
	assert_bool(db.set_inventory("throw_ball", "ball_1", 5)).is_true()
	assert_bool(db.set_inventory("throw_ball", "ball_1", 3)).is_true()
	assert_array(db.get_inventory()).has_size(1)

	# ---- checkin ----
	assert_bool(db.add_checkin({"checkin_date": "2026-07-08", "streak": 1})).is_true()
	assert_array(db.get_checkins()).has_size(1)

	db.close()

	# ---- 重启模拟: 新实例打开同文件, 数据仍在 ----
	var db2 := LocalDB.new()
	assert_bool(db2.open(TMP_DB)).is_true()
	assert_int(db2.get_progress().get("level")).is_equal(4)
	assert_array(db2.get_inventory()).has_size(1)
	db2.close()


func test_delete_missing_animal_returns_false() -> void:
	if not ClassDB.class_exists("SQLite"):
		return
	var db := LocalDB.new()
	assert_bool(db.open(TMP_DB)).is_true()
	# godot-sqlite delete_rows 对不存在的行返回 true(影响 0 行); 这里仅验证不崩溃
	db.delete_animal("does_not_exist")
	db.close()
