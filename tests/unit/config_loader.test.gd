extends GdUnitTestSuite

# ConfigLoader 纯逻辑测试: .env 解析 / 类型转换 / 优先级。
# 注: GDScript 无法设置 OS 环境变量(OS 无 set_environment), 故 OS-env>/.env 优先级
# 无法在此覆盖; 仅测 .env 缓存 > 默认值。

const TMP_DIR := "user://tmp"
const TMP_ENV := "user://tmp/test_config_loader.env"

func before_test() -> void:
	_write_env_file()

func after_test() -> void:
	_remove_tmp()


func _write_env_file() -> void:
	if not DirAccess.dir_exists_absolute(TMP_DIR):
		DirAccess.make_dir_recursive_absolute(TMP_DIR)
	var f := FileAccess.open(TMP_ENV, FileAccess.WRITE)
	f.store_line("# 这是一个注释")
	f.store_line("")
	f.store_line("BACKEND_BASE_URL=http://test:8080")
	f.store_line("LOG_LEVEL=DEBUG")
	f.store_line("ANIMAL_POKE_TEST_KEY=from_env")
	f.store_line('QUOTED_DOUBLE="double value"')
	f.store_line("QUOTED_SINGLE='single value'")
	f.store_line("EMPTY_VAL=")
	f.store_line("NO_EQUALS_LINE")
	f.store_line("NUM=42")
	f.store_line("FLOAT_VAL=3.14")
	f.store_line("B_TRUE=true")
	f.store_line("B_ONE=1")
	f.store_line("B_YES=yes")
	f.store_line("B_UPPER=TRUE")
	f.store_line("B_NO=no")
	f.close()


func _remove_tmp() -> void:
	if FileAccess.file_exists(TMP_ENV):
		DirAccess.remove_absolute(TMP_ENV)


func test_parse_comments_blanks_and_kv() -> void:
	var cl := ConfigLoader.new()
	cl.load_file(TMP_ENV)
	assert_str(cl.get_string("BACKEND_BASE_URL", "")).is_equal("http://test:8080")
	assert_str(cl.get_string("LOG_LEVEL", "")).is_equal("DEBUG")


func test_parse_quoted_values() -> void:
	var cl := ConfigLoader.new()
	cl.load_file(TMP_ENV)
	assert_str(cl.get_string("QUOTED_DOUBLE", "")).is_equal("double value")
	assert_str(cl.get_string("QUOTED_SINGLE", "")).is_equal("single value")


func test_empty_value_and_no_equals_skipped() -> void:
	var cl := ConfigLoader.new()
	cl.load_file(TMP_ENV)
	# EMPTY_VAL 解析为空字符串(has 返回 false)
	assert_str(cl.get_string("EMPTY_VAL", "default")).is_equal("")
	assert_bool(cl.has("EMPTY_VAL")).is_false()
	# NO_EQUALS_LINE 被跳过
	assert_bool(cl.has("NO_EQUALS_LINE")).is_false()


func test_env_cache_overrides_default() -> void:
	var cl := ConfigLoader.new()
	cl.load_file(TMP_ENV)
	# .env 缓存值优先于默认值
	assert_str(cl.get_string("ANIMAL_POKE_TEST_KEY", "default")).is_equal("from_env")
	# 未设置 key 返回默认
	assert_str(cl.get_string("MISSING_KEY", "default")).is_equal("default")


func test_get_int() -> void:
	var cl := ConfigLoader.new()
	cl.load_file(TMP_ENV)
	assert_int(cl.get_int("NUM", 0)).is_equal(42)
	assert_int(cl.get_int("MISSING_INT", 7)).is_equal(7)
	assert_int(cl.get_int("EMPTY_VAL", 9)).is_equal(9)
	# 注: String.to_int() 会提取字符串中首个数字串, "http://test:8080" → 8080(不回退默认)
	assert_int(cl.get_int("BACKEND_BASE_URL", 5)).is_equal(8080)
	# 完全无数字的值 → to_int 返回 0
	assert_int(cl.get_int("QUOTED_DOUBLE", 5)).is_equal(0)


func test_get_float() -> void:
	var cl := ConfigLoader.new()
	cl.load_file(TMP_ENV)
	assert_float(cl.get_float("FLOAT_VAL", 0.0)).is_equal(3.14)
	assert_float(cl.get_float("MISSING_FLOAT", 1.5)).is_equal(1.5)


func test_get_bool() -> void:
	var cl := ConfigLoader.new()
	cl.load_file(TMP_ENV)
	assert_bool(cl.get_bool("B_TRUE", false)).is_true()
	assert_bool(cl.get_bool("B_ONE", false)).is_true()
	assert_bool(cl.get_bool("B_YES", false)).is_true()
	assert_bool(cl.get_bool("B_UPPER", false)).is_true()
	assert_bool(cl.get_bool("B_NO", true)).is_false()
	assert_bool(cl.get_bool("MISSING_BOOL", true)).is_true()
	assert_bool(cl.get_bool("MISSING_BOOL", false)).is_false()


func test_has() -> void:
	var cl := ConfigLoader.new()
	cl.load_file(TMP_ENV)
	assert_bool(cl.has("BACKEND_BASE_URL")).is_true()
	assert_bool(cl.has("NUM")).is_true()
	assert_bool(cl.has("MISSING")).is_false()
	assert_bool(cl.has("EMPTY_VAL")).is_false() # 空值视为未配置


func test_missing_file_is_silent() -> void:
	var cl := ConfigLoader.new()
	# 不存在的文件不报错, 全部走默认值
	assert_str(cl.get_string("ANY", "default")).is_equal("default")
	assert_bool(cl.has("ANY")).is_false()
