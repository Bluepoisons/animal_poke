extends GdUnitTestSuite

# LocalDB 纯逻辑测试: _sql_escape / _read_file / 未 open 时的 guard 路径。
# CRUD 集成测试见 tests/integration/save_db_integration.test.gd(依赖 godot-sqlite 插件)。

const TMP_DB := "test_unit_localdb.db"
const TMP_DB_PATH := "user://test_unit_localdb.db"
# 注意: 不清理 user://.dbkey — 它是与真实存档(animal_poke.db)共享的加密密钥,
# 删除会导致下次启动 SaveManager 无法打开已有库。仅清理测试用 db 文件。


func test_sql_escape() -> void:
	var db := LocalDB.new()
	assert_str(db._sql_escape("plain")).is_equal("plain")
	assert_str(db._sql_escape("it's")).is_equal("it''s")
	assert_str(db._sql_escape("a'b'c")).is_equal("a''b''c")
	assert_str(db._sql_escape("")).is_equal("")


func test_read_file_existing() -> void:
	var db := LocalDB.new()
	var sql := db._read_file("res://scripts/core/schema.sql")
	assert_bool(sql.length() > 0).is_true()
	assert_bool(sql.contains("CREATE TABLE")).is_true()


func test_read_file_missing_returns_empty() -> void:
	var db := LocalDB.new()
	assert_str(db._read_file("res://does_not_exist.sql")).is_equal("")


func test_guard_paths_when_not_open() -> void:
	var db := LocalDB.new()
	assert_bool(db.is_open()).is_false()

	# 所有 CRUD 在未 open 时走 _ensure guard, 返回空/false, 不触碰 _db
	assert_bool(db.upsert_animal({"uuid": "x"})).is_false()
	assert_dict(db.get_animal("x")).is_empty()
	assert_array(db.get_all_animals()).is_empty()
	assert_bool(db.delete_animal("x")).is_false()

	assert_bool(db.upsert_progress({"level": 1})).is_false()
	assert_dict(db.get_progress()).is_empty()

	assert_bool(db.set_inventory("t", "i", 1)).is_false()
	assert_array(db.get_inventory()).is_empty()

	assert_bool(db.add_checkin({})).is_false()
	assert_array(db.get_checkins()).is_empty()


func test_open_close_when_plugin_available() -> void:
	if not ClassDB.class_exists("SQLite"):
		print("[db_logic] godot-sqlite 未加载, 跳过 open/close 测试(见集成测试)")
		return

	var db := LocalDB.new()
	_cleanup_files()
	assert_bool(db.open(TMP_DB)).is_true()
	assert_bool(db.is_open()).is_true()
	db.close()
	assert_bool(db.is_open()).is_false()
	_cleanup_files()


func _cleanup_files() -> void:
	if FileAccess.file_exists(TMP_DB_PATH):
		DirAccess.remove_absolute(TMP_DB_PATH)


func after_test() -> void:
	_cleanup_files()
