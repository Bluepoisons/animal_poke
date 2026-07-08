extends Node

## StaminaManager — 体力系统 (M1-MVP)
## 管理体力资源的消耗、自然恢复和持久化。
## 参考: 3.4 体力系统、5.5 等级-体力对照表

signal stamina_changed(current: int, max_stamina: int)
signal stamina_insufficient(required: int, current: int)

const BASE_STAMINA_MAX: int = 120
const STAMINA_PER_LEVEL: int = 14
const CAPTURE_COST: int = 20
const DISPATCH_COST: int = 20
const RECOVERY_PER_HOUR: int = 10
const RECOVERY_INTERVAL_SEC: int = 360  # 每 6 分钟 +1 点 = 360 秒

var _current_stamina: int = 120
var _max_stamina: int = 120
var _last_update_unix: int = 0


func _ready() -> void:
	_load_and_recover()


## 加载存档并计算离线恢复
func _load_and_recover() -> void:
	var sm: Node = _get_save_manager()
	if sm == null:
		return

	var progress: Dictionary = sm.call("load_progress")

	if progress.is_empty():
		_current_stamina = BASE_STAMINA_MAX
		_max_stamina = BASE_STAMINA_MAX
		_last_update_unix = int(Time.get_unix_time_from_system())
		_save()
		return

	_current_stamina = progress.get("stamina", BASE_STAMINA_MAX)
	var saved_at: String = progress.get("stamina_updated_at", "")
	var level: int = progress.get("level", 1)
	_max_stamina = _get_max_stamina_for_level(level)

	if not saved_at.is_empty() and saved_at.is_valid_int():
		_last_update_unix = int(saved_at)
	else:
		_last_update_unix = int(Time.get_unix_time_from_system())

	_recover()
	_save()


## 根据离线时间计算自然恢复
func _recover() -> void:
	var now_unix: int = int(Time.get_unix_time_from_system())
	var elapsed: int = now_unix - _last_update_unix
	if elapsed <= 0:
		return

	var recovered: int = elapsed / RECOVERY_INTERVAL_SEC
	if recovered > 0:
		var old_stamina := _current_stamina
		_current_stamina = min(_current_stamina + recovered, _max_stamina)
		if _current_stamina != old_stamina:
			_last_update_unix = now_unix


## 立即计算并返回当前体力(考虑实时恢复, 不写库)
func _get_real_time_stamina() -> int:
	var now_unix: int = int(Time.get_unix_time_from_system())
	var elapsed: int = now_unix - _last_update_unix
	if elapsed <= 0:
		return _current_stamina
	var recovered: int = elapsed / RECOVERY_INTERVAL_SEC
	return min(_current_stamina + recovered, _max_stamina)


# 等级-体力上限对照表 (参考 5.5)
const STAMINA_MAX_TABLE := {
	1: 120, 2: 134, 3: 148, 4: 162, 5: 176,
	6: 190, 7: 204, 8: 218, 9: 232, 10: 240,
}


func _get_max_stamina_for_level(level: int) -> int:
	# 查表优先, 越界时用最高档
	if level >= 1 and level <= 10:
		return STAMINA_MAX_TABLE[level]
	if level > 10:
		return STAMINA_MAX_TABLE[10]
	return BASE_STAMINA_MAX


## 是否有足够体力执行操作
func can_capture() -> bool:
	return _get_real_time_stamina() >= CAPTURE_COST


func can_dispatch() -> bool:
	return _get_real_time_stamina() >= DISPATCH_COST


## 消耗体力用于捕获。返回是否成功。
func consume_for_capture() -> bool:
	_recover()  # 先结算实时恢复
	if _current_stamina < CAPTURE_COST:
		stamina_insufficient.emit(CAPTURE_COST, _current_stamina)
		return false
	_current_stamina -= CAPTURE_COST
	_save()
	stamina_changed.emit(_current_stamina, _max_stamina)
	return true


## 消耗体力用于派遣。返回是否成功。
func consume_for_dispatch() -> bool:
	_recover()
	if _current_stamina < DISPATCH_COST:
		stamina_insufficient.emit(DISPATCH_COST, _current_stamina)
		return false
	_current_stamina -= DISPATCH_COST
	_save()
	stamina_changed.emit(_current_stamina, _max_stamina)
	return true


## 恢复满体力(升级时调用)
func restore_to_full() -> void:
	_current_stamina = _max_stamina
	_save()
	stamina_changed.emit(_current_stamina, _max_stamina)


## 升级时更新体力上限并回满
func on_level_up(new_level: int) -> void:
	_max_stamina = _get_max_stamina_for_level(new_level)
	restore_to_full()


## 购买体力药剂(商店调用)
func purchase_stamina(amount: int) -> void:
	_current_stamina = min(_current_stamina + amount, _max_stamina)
	_save()
	stamina_changed.emit(_current_stamina, _max_stamina)


func get_current() -> int:
	return _current_stamina


func get_max() -> int:
	return _max_stamina


## 持久化到 player_progress 表
func _save() -> void:
	var sm: Node = _get_save_manager()
	if sm == null:
		return
	sm.call("save_progress", {
		"stamina": _current_stamina,
		"stamina_updated_at": str(int(Time.get_unix_time_from_system())),
	})


func _get_save_manager():
	return get_node_or_null("/root/SaveManager")
