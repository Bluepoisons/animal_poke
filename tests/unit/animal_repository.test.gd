extends GdUnitTestSuite

# AnimalRepository 单元测试: UUID 生成 / 校验 / 隐私剥离 / CRUD 纯逻辑。
# 依赖 godot-sqlite 插件做 CRUD; 未加载则仅跑不触库的纯逻辑用例。

const TMP_DB := "test_unit_animal_repo.db"
const TMP_DB_PATH := "user://test_unit_animal_repo.db"


func before_test() -> void:
	_cleanup()


func after_test() -> void:
	_cleanup()


func _cleanup() -> void:
	if FileAccess.file_exists(TMP_DB_PATH):
		DirAccess.remove_absolute(TMP_DB_PATH)


func _new_repo() -> AnimalRepository:
	var db := LocalDB.new()
	assert_bool(db.open(TMP_DB)).is_true()
	return AnimalRepository.new(db)


# ---------- UUID 生成 ----------

func test_uuid_format() -> void:
	var repo := AnimalRepository.new(null)
	var uuid := repo._generate_uuid()
	# 8-4-4-4-12 = 36 字符(含 4 个连字符)
	assert_int(uuid.length()).is_equal(36)
	# version 4: 第 3 段以 4 开头
	assert_str(uuid.substr(14, 1)).is_equal("4")
	# variant: 第 4 段首字符为 8/9/a/b
	var v := uuid.substr(19, 1)
	assert_bool(v == "8" or v == "9" or v == "a" or v == "b").is_true()


func test_uuid_generation_uniqueness() -> void:
	var repo := AnimalRepository.new(null)
	var seen := {}
	for i in 1000:
		var u := repo._generate_uuid()
		assert_bool(seen.has(u)).is_false()
		seen[u] = true
	assert_int(seen.size()).is_equal(1000)


# ---------- 校验 ----------

func test_validation_rejects_empty_species() -> void:
	if not ClassDB.class_exists("SQLite"):
		return
	var repo := _new_repo()
	var uuid := repo.save_captured_animal({"species": "", "rarity": 1})
	assert_str(uuid).is_equal("")
	repo._db.close()


func test_validation_rejects_missing_species() -> void:
	if not ClassDB.class_exists("SQLite"):
		return
	var repo := _new_repo()
	# 完全不传 species
	var uuid := repo.save_captured_animal({"rarity": 1})
	assert_str(uuid).is_equal("")
	repo._db.close()


func test_validation_rejects_bad_rarity() -> void:
	if not ClassDB.class_exists("SQLite"):
		return
	var repo := _new_repo()
	assert_str(repo.save_captured_animal({"species": "cat", "rarity": -1})).is_equal("")
	assert_str(repo.save_captured_animal({"species": "cat", "rarity": 5})).is_equal("")
	assert_str(repo.save_captured_animal({"species": "cat", "rarity": 99})).is_equal("")
	repo._db.close()


func test_validation_rejects_attr_out_of_range() -> void:
	if not ClassDB.class_exists("SQLite"):
		return
	var repo := _new_repo()
	# HP 超上限(5.2: 50-500)
	assert_str(repo.save_captured_animal({"species": "cat", "rarity": 1, "attr_hp": 9999})).is_equal("")
	# HP 低于下限(非 0: 0 = 未生成, 允许)
	assert_str(repo.save_captured_animal({"species": "cat", "rarity": 1, "attr_hp": 10})).is_equal("")
	# ATK 越界(10-120); 0 = 未生成允许, 故用 -1 测试拒绝
	assert_str(repo.save_captured_animal({"species": "cat", "rarity": 1, "attr_atk": -1})).is_equal("")
	# SPD 越界(1-100)
	assert_str(repo.save_captured_animal({"species": "cat", "rarity": 1, "attr_spd": 200})).is_equal("")
	repo._db.close()


func test_validation_accepts_zero_as_pending() -> void:
	# 0 = "尚未生成"(待 M2 云端 VLM/LLM), 允许通过校验
	if not ClassDB.class_exists("SQLite"):
		return
	var repo := _new_repo()
	var uuid := repo.save_captured_animal({
		"species": "cat", "rarity": 1,
		"attr_hp": 0, "attr_atk": 0, "attr_def": 0, "attr_spd": 0,
	})
	assert_bool(uuid.length() == 36).is_true()
	repo._db.close()


func test_validation_accepts_valid_animal() -> void:
	if not ClassDB.class_exists("SQLite"):
		return
	var repo := _new_repo()
	var uuid := repo.save_captured_animal({
		"species": "cat", "rarity": 2,
		"attr_hp": 200, "attr_atk": 50, "attr_def": 30, "attr_spd": 80,
	})
	assert_bool(uuid.length() == 36).is_true()
	repo._db.close()


# ---------- 隐私: photo 相关键剥离(4.5) ----------

func test_photo_keys_stripped() -> void:
	if not ClassDB.class_exists("SQLite"):
		return
	var repo := _new_repo()
	var uuid := repo.save_captured_animal({
		"species": "cat", "rarity": 1,
		"photo": "base64data...", "raw_frame": PackedByteArray([1, 2, 3]),
		"image": "data", "thumbnail": "thumb",
	})
	assert_bool(uuid.length() == 36).is_true()
	var got := repo.get_animal(uuid)
	assert_bool(got.has("photo")).is_false()
	assert_bool(got.has("raw_frame")).is_false()
	assert_bool(got.has("image")).is_false()
	assert_bool(got.has("thumbnail")).is_false()
	assert_str(got.get("species", "")).is_equal("cat")
	repo._db.close()


# ---------- 时间戳自动填充 ----------

func test_save_captured_fills_timestamps() -> void:
	if not ClassDB.class_exists("SQLite"):
		return
	var repo := _new_repo()
	var uuid := repo.save_captured_animal({"species": "cat", "rarity": 1})
	var got := repo.get_animal(uuid)
	assert_str(got.get("created_at", "")).is_not_equal("")
	assert_str(got.get("generated_at", "")).is_not_equal("")
	assert_str(got.get("updated_at", "")).is_not_equal("")
	# ISO8601 格式含 T 与 Z
	assert_bool(got["created_at"].contains("T")).is_true()
	assert_bool(got["created_at"].ends_with("Z")).is_true()
	repo._db.close()


# ---------- CRUD ----------

func test_get_getall_delete() -> void:
	if not ClassDB.class_exists("SQLite"):
		return
	var repo := _new_repo()
	var u1 := repo.save_captured_animal({"species": "cat", "rarity": 1})
	var u2 := repo.save_captured_animal({"species": "goose", "rarity": 2})
	var u3 := repo.save_captured_animal({"species": "dog", "rarity": 3})
	assert_bool(u1 != "" and u2 != "" and u3 != "").is_true()

	assert_array(repo.get_all_animals()).has_size(3)

	var got := repo.get_animal(u2)
	assert_str(got.get("uuid", "")).is_equal(u2)
	assert_str(got.get("species", "")).is_equal("goose")

	assert_bool(repo.delete_animal(u2)).is_true()
	assert_array(repo.get_all_animals()).has_size(2)
	assert_dict(repo.get_animal(u2)).is_empty()
	repo._db.close()


func test_upsert_same_uuid_updates() -> void:
	if not ClassDB.class_exists("SQLite"):
		return
	var repo := _new_repo()
	var uuid := repo.save_captured_animal({"species": "cat", "rarity": 1})
	assert_array(repo.get_all_animals()).has_size(1)

	# 同 uuid 二次 save, 应更新而非新增
	var data := repo.get_animal(uuid)
	data["rarity"] = 4
	data["attr_hp"] = 300
	var uuid2 := repo.save_captured_animal(data)
	assert_str(uuid2).is_equal(uuid)
	assert_array(repo.get_all_animals()).has_size(1)
	var got := repo.get_animal(uuid)
	assert_int(got.get("rarity")).is_equal(4)
	assert_int(got.get("attr_hp")).is_equal(300)
	repo._db.close()


# ---------- 数据库未就绪时的 guard ----------

func test_guard_when_db_null() -> void:
	var repo := AnimalRepository.new(null)
	assert_str(repo.save_captured_animal({"species": "cat", "rarity": 1})).is_equal("")
	assert_dict(repo.get_animal("x")).is_empty()
	assert_array(repo.get_all_animals()).is_empty()
	assert_bool(repo.delete_animal("x")).is_false()


# ---------- _strip_photo_keys 纯逻辑 ----------

func test_strip_photo_keys_returns_clean_copy() -> void:
	var repo := AnimalRepository.new(null)
	var clean := repo._strip_photo_keys({
		"species": "cat", "photo": "x", "IMAGE": "y", "name": "n",
	})
	assert_bool(clean.has("species")).is_true()
	assert_bool(clean.has("photo")).is_false()
	# 键名大小写不敏感(转 lower 判断)
	assert_bool(clean.has("IMAGE")).is_false()
	assert_bool(clean.has("name")).is_true()


func test_strip_photo_keys_does_not_mutate_input() -> void:
	var repo := AnimalRepository.new(null)
	var orig := {"species": "cat", "photo": "x"}
	var _clean := repo._strip_photo_keys(orig)
	# 原 dict 不应被修改(内部 duplicate)
	assert_bool(orig.has("photo")).is_true()
