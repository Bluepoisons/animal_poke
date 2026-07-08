extends GdUnitTestSuite

# 集成测试 — 直接验证 issue #2 [M1] 本地存储验收标准:
#   "动物数据可持久化, 重启不丢失"
#
# 场景: 捕获(写入) → 关闭进程 → 重启(新实例打开同库) → 数据仍在。
# 依赖 godot-sqlite 插件; 未加载则跳过。
# 用独立临时 db 文件, 不污染真实存档(animal_poke.db)。不删 user://.dbkey。

const TMP_DB := "test_integration_persistence.db"
const TMP_DB_PATH := "user://test_integration_persistence.db"


func before_test() -> void:
	_cleanup()


func after_test() -> void:
	_cleanup()


func _cleanup() -> void:
	if FileAccess.file_exists(TMP_DB_PATH):
		DirAccess.remove_absolute(TMP_DB_PATH)


# ---------- 核心: 捕获 → 关闭 → 重启 → 数据仍在 ----------

func test_capture_restart_persistence() -> void:
	if not ClassDB.class_exists("SQLite"):
		print("[animal_persistence] godot-sqlite 未加载, 跳过")
		return

	# --- 会话 1: 捕获 3 只动物 + 写进度 ---
	var db1 := LocalDB.new()
	assert_bool(db1.open(TMP_DB)).is_true()
	var repo1 := AnimalRepository.new(db1)

	var u1 := repo1.save_captured_animal({
		"species": "cat", "breed": "tabby", "rarity": 1,
		"attr_hp": 120, "attr_atk": 40, "attr_def": 20, "attr_spd": 70,
		"lat": 29.87, "lng": 121.54,
		"inference_request_id": "req-001", "model_version": "vlm-v1",
		"capture_method": "throw",
	})
	var u2 := repo1.save_captured_animal({
		"species": "cat", "breed": "ragdoll", "rarity": 3,
		"attr_hp": 200, "attr_atk": 60, "attr_def": 35, "attr_spd": 90,
	})
	var u3 := repo1.save_captured_animal({
		"species": "cat", "breed": "orange", "rarity": 0,
	})
	assert_bool(u1 != "" and u2 != "" and u3 != "").is_true()

	# 写玩家进度(等级/金币/体力)
	assert_bool(db1.upsert_progress({"level": 2, "coins": 150, "stamina": 100})).is_true()

	# 快照捕获数据(用于重启后比对)
	var snap1 := repo1.get_all_animals()
	var prog1 := db1.get_progress()

	# --- 模拟进程退出 ---
	db1.close()
	assert_bool(db1.is_open()).is_false()

	# --- 会话 2: 重启, 新实例打开同一 db 文件 ---
	var db2 := LocalDB.new()
	assert_bool(db2.open(TMP_DB)).is_true()
	var repo2 := AnimalRepository.new(db2)

	# 验收: 重启不丢
	var snap2 := repo2.get_all_animals()
	assert_array(snap2).has_size(3)

	# 每只动物的 uuid / species / rarity / 属性 / 坐标 / 推理ID 一致
	var by_uuid := {}
	for row in snap2:
		by_uuid[row["uuid"]] = row

	assert_bool(by_uuid.has(u1)).is_true()
	var a1: Dictionary = by_uuid[u1]
	assert_str(a1["species"]).is_equal("cat")
	assert_str(a1["breed"]).is_equal("tabby")
	assert_int(a1["rarity"]).is_equal(1)
	assert_int(a1["attr_hp"]).is_equal(120)
	assert_int(a1["attr_atk"]).is_equal(40)
	assert_float(a1["lat"]).is_equal(29.87)
	assert_float(a1["lng"]).is_equal(121.54)
	assert_str(a1["inference_request_id"]).is_equal("req-001")
	assert_str(a1["model_version"]).is_equal("vlm-v1")
	assert_str(a1["capture_method"]).is_equal("throw")
	assert_str(a1["created_at"]).is_not_equal("")

	assert_bool(by_uuid.has(u2)).is_true()
	assert_int(by_uuid[u2]["rarity"]).is_equal(3)

	assert_bool(by_uuid.has(u3)).is_true()
	assert_int(by_uuid[u3]["rarity"]).is_equal(0)

	# 进度也重启不丢
	var prog2 := db2.get_progress()
	assert_int(prog2["level"]).is_equal(2)
	assert_int(prog2["coins"]).is_equal(150)
	assert_int(prog2["stamina"]).is_equal(100)

	db2.close()

	# 引用 snap1/snap1 防止静态分析告警(实际比对已在 snap2 完成)
	assert_array(snap1).has_size(3)
	assert_dict(prog1).is_not_empty()


# ---------- 隐私: photo 不落盘(4.5), 重启后仍不存在 ----------

func test_photo_not_persisted_across_restart() -> void:
	if not ClassDB.class_exists("SQLite"):
		return

	var db1 := LocalDB.new()
	assert_bool(db1.open(TMP_DB)).is_true()
	var repo1 := AnimalRepository.new(db1)

	var uuid := repo1.save_captured_animal({
		"species": "cat", "rarity": 1,
		"photo": "should_not_persist", "raw_frame": PackedByteArray([1, 2, 3]),
		"thumbnail": "thumb_data",
	})
	assert_bool(uuid != "").is_true()

	# 当前会话: 已剥离
	var a1 := repo1.get_animal(uuid)
	assert_bool(a1.has("photo")).is_false()
	assert_bool(a1.has("raw_frame")).is_false()
	assert_bool(a1.has("thumbnail")).is_false()

	db1.close()

	# 重启后: 仍不存在(从未落库)
	var db2 := LocalDB.new()
	assert_bool(db2.open(TMP_DB)).is_true()
	var repo2 := AnimalRepository.new(db2)
	var a2 := repo2.get_animal(uuid)
	assert_bool(a2.has("photo")).is_false()
	assert_bool(a2.has("raw_frame")).is_false()
	assert_bool(a2.has("thumbnail")).is_false()
	assert_str(a2.get("species", "")).is_equal("cat")
	db2.close()


# ---------- 多次会话累积: 不覆盖不丢 ----------

func test_multiple_sessions_accumulate() -> void:
	if not ClassDB.class_exists("SQLite"):
		return

	# 会话 1: 写 2 只
	var db1 := LocalDB.new()
	assert_bool(db1.open(TMP_DB)).is_true()
	var repo1 := AnimalRepository.new(db1)
	var u1 := repo1.save_captured_animal({"species": "cat", "rarity": 1})
	var u2 := repo1.save_captured_animal({"species": "cat", "rarity": 2})
	db1.close()

	# 会话 2: 再写 2 只(独立进程实例)
	var db2 := LocalDB.new()
	assert_bool(db2.open(TMP_DB)).is_true()
	var repo2 := AnimalRepository.new(db2)
	var u3 := repo2.save_captured_animal({"species": "cat", "rarity": 3})
	var u4 := repo2.save_captured_animal({"species": "cat", "rarity": 4})
	db2.close()

	# 会话 3: 打开应见 4 只
	var db3 := LocalDB.new()
	assert_bool(db3.open(TMP_DB)).is_true()
	var repo3 := AnimalRepository.new(db3)
	var all := repo3.get_all_animals()
	assert_array(all).has_size(4)

	var uuids := {}
	for row in all:
		uuids[row["uuid"]] = true
	assert_bool(uuids.has(u1) and uuids.has(u2) and uuids.has(u3) and uuids.has(u4)).is_true()
	db3.close()


# ---------- UUID 跨会话唯一(不碰撞) ----------

func test_uuid_unique_across_sessions() -> void:
	if not ClassDB.class_exists("SQLite"):
		return

	var uuids := {}
	# 3 次独立会话各写 5 只
	for session in 3:
		var db := LocalDB.new()
		assert_bool(db.open(TMP_DB)).is_true()
		var repo := AnimalRepository.new(db)
		for i in 5:
			var u := repo.save_captured_animal({"species": "cat", "rarity": i % 5})
			assert_bool(u != "").is_true()
			assert_bool(uuids.has(u)).is_false()
			uuids[u] = true
		db.close()

	# 共 15 只, uuid 全唯一
	assert_int(uuids.size()).is_equal(15)

	# 最终库内 15 只
	var dbf := LocalDB.new()
	assert_bool(dbf.open(TMP_DB)).is_true()
	var repo_final := AnimalRepository.new(dbf)
	assert_array(repo_final.get_all_animals()).has_size(15)
	dbf.close()
