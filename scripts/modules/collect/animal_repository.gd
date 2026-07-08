class_name AnimalRepository
extends RefCounted

## AnimalRepository — 动物元数据领域层(M8 持久化部分)
## 捕获后落库的入口: 生成 UUID + 设备 ID + 时间戳, 校验后委托 LocalDB 持久化。
## 参考: 4.4 同步方案(UUID/设备ID/时间戳/推理请求ID)、5.2 六维属性范围、4.5 隐私(原始照片不落盘)。
##
## 隐私强制(4.5): 原始照片经后端转发 VLM 即时销毁, 本地不持久化。
## 本层在 upsert 前剥离任何 photo/image/raw_frame/thumbnail 键, 防止误落盘。
##
## 同步(上传后端 /sync/animal, MB4)不在本类范围, 见 sync_service.gd(待 MB4 完成后实现)。

# 六维属性范围(5.2)
const ATTR_HP_MIN := 50
const ATTR_HP_MAX := 500
const ATTR_ATK_MIN := 10
const ATTR_ATK_MAX := 120
const ATTR_DEF_MIN := 5
const ATTR_DEF_MAX := 80
const ATTR_SPD_MIN := 1
const ATTR_SPD_MAX := 100

# 稀有度范围(0-4 对应 灰/绿/蓝/紫/金, 见 5.1 + scripts/ui/rarity.gd)
const RARITY_MIN := 0
const RARITY_MAX := 4

# 隐私: 严禁落盘本地的 photo 相关键(4.5 原始照片不落盘)
const _PHOTO_KEYS := {
	"photo": true, "image": true, "raw_frame": true, "thumbnail": true,
	"raw_image": true, "frame": true, "picture": true, "img": true,
}

var _db: LocalDB  # 注入: 生产用 SaveManager.get_db(), 测试用临时 LocalDB


func _init(db: LocalDB = null) -> void:
	_db = db


## 捕获后保存动物元数据。data 字段见 schema.sql(animals)。
## 返回生成的 uuid; 校验失败或落库失败返回 ""。
func save_captured_animal(data: Dictionary) -> String:
	if _db == null or not _db.is_open():
		push_error("[AnimalRepository] 数据库未就绪, 无法保存动物。")
		return ""

	var clean := _strip_photo_keys(data)

	# uuid: 已有则复用(upsert 更新), 否则生成
	var uuid := str(clean.get("uuid", ""))
	if uuid == "":
		uuid = _generate_uuid()
		clean["uuid"] = uuid

	# 校验(校验失败不落库)
	if not _validate(clean):
		return ""

	# 时间戳(ISO8601 UTC)
	var now := _now_iso8601()
	if str(clean.get("created_at", "")) == "":
		clean["created_at"] = now
	if str(clean.get("generated_at", "")) == "":
		clean["generated_at"] = now
	clean["updated_at"] = now

	if _db.upsert_animal(clean):
		return uuid
	push_error("[AnimalRepository] 落库失败: %s" % uuid)
	return ""


## 按 uuid 读取单只动物(图鉴详情页用)
func get_animal(uuid: String) -> Dictionary:
	if _db == null or not _db.is_open():
		return {}
	return _db.get_animal(uuid)


## 读取全部动物(图鉴列表用, 离线可)
func get_all_animals() -> Array:
	if _db == null or not _db.is_open():
		return []
	return _db.get_all_animals()


## 删除一只动物(图鉴管理/测试用)
func delete_animal(uuid: String) -> bool:
	if _db == null or not _db.is_open():
		return false
	return _db.delete_animal(uuid)


# ---------- UUID 生成(RFC4122 v4 随机) ----------

func _generate_uuid() -> String:
	# 16 字节随机, 第 6 字节高 4 位设 0100(v4), 第 8 字节高 2 位设 10(variant)
	var b := PackedByteArray()
	b.resize(16)
	for i in 16:
		b[i] = randi() % 256
	b[6] = (b[6] & 0x0F) | 0x40  # version 4
	b[8] = (b[8] & 0x3F) | 0x80  # variant RFC4122
	var hex := b.hex_encode()
	# 8-4-4-4-12
	return "%s-%s-%s-%s-%s" % [hex.substr(0, 8), hex.substr(8, 4), hex.substr(12, 4), hex.substr(16, 4), hex.substr(20, 12)]


# ---------- 校验 ----------

func _validate(data: Dictionary) -> bool:
	# species 非空
	if str(data.get("species", "")).strip_edges() == "":
		push_warning("[AnimalRepository] 校验失败: species 不能为空。")
		return false

	# 稀有度 0-4
	var rarity := int(data.get("rarity", 0))
	if rarity < RARITY_MIN or rarity > RARITY_MAX:
		push_warning("[AnimalRepository] 校验失败: rarity=%d 越界(%d-%d)。" % [rarity, RARITY_MIN, RARITY_MAX])
		return false

	# 六维属性范围(5.2), 仅校验显式提供的字段
	if not _in_range_or_absent(data, "attr_hp", ATTR_HP_MIN, ATTR_HP_MAX):
		return false
	if not _in_range_or_absent(data, "attr_atk", ATTR_ATK_MIN, ATTR_ATK_MAX):
		return false
	if not _in_range_or_absent(data, "attr_def", ATTR_DEF_MIN, ATTR_DEF_MAX):
		return false
	if not _in_range_or_absent(data, "attr_spd", ATTR_SPD_MIN, ATTR_SPD_MAX):
		return false
	return true


func _in_range_or_absent(data: Dictionary, key: String, lo: int, hi: int) -> bool:
	if not data.has(key):
		return true
	var v := int(data[key])
	# 0 = "尚未生成"(schema 默认值, 待 M2 云端 VLM/LLM 生成真实数值), 跳过范围校验。
	if v == 0:
		return true
	if v < lo or v > hi:
		push_warning("[AnimalRepository] 校验失败: %s=%d 越界(%d-%d)。" % [key, v, lo, hi])
		return false
	return true


# ---------- 隐私: 剥离 photo 相关键(4.5) ----------

func _strip_photo_keys(data: Dictionary) -> Dictionary:
	var clean := data.duplicate(true)
	var removed := []
	for key in clean.keys():
		if _PHOTO_KEYS.has(str(key).to_lower()):
			clean.erase(key)
			removed.append(key)
	if not removed.is_empty():
		push_warning("[AnimalRepository] 已剥离 photo 相关键(4.5 不落盘): %s" % str(removed))
	return clean


# ---------- 工具 ----------

func _now_iso8601() -> String:
	# UTC, 如 2026-07-08T12:34:56Z
	var dt := Time.get_datetime_dict_from_system(false)  # UTC
	return "%04d-%02d-%02dT%02d:%02d:%02dZ" % [dt.year, dt.month, dt.day, dt.hour, dt.minute, dt.second]
