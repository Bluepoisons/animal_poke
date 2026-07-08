extends Node

## SceneManager — 场景切换栈(单例)
## 维护场景栈, 支持 push(压入新场景) / pop(返回上一场景) / replace(替换栈顶) /
## reset_to(清空栈并跳转)。用于 发现 → 捕获 → 收藏 等流程的场景导航。
##
## 注意: 实际场景文件在 MVP 任务 M13 落地, 本单例先提供导航能力。
## 启动主场景由 project.godot 的 application/run/main_scene 决定(MVP 暂未设置)。

signal scene_changed(scene_path: String)

var _stack: Array[String] = []
var _current: String = ""


func _ready() -> void:
	pass


## 当前场景资源路径
func current_scene_path() -> String:
	return _current


## 栈深度
func stack_size() -> int:
	return _stack.size()


## 替换当前场景(不压栈)。用于主菜单 → 业务入口等无需返回的场景。
func replace(scene_path: String) -> void:
	var err := get_tree().change_scene_to_file(scene_path)
	if err != OK:
		push_error("[SceneManager] 切换场景失败: %s (err=%d)" % [scene_path, err])
		return
	if _stack.size() > 0:
		_stack[_stack.size() - 1] = scene_path
	else:
		_stack.append(scene_path)
	_current = scene_path
	scene_changed.emit(scene_path)


## 压入新场景(当前场景保留在栈中, 可 pop 返回)
func push(scene_path: String) -> void:
	var err := get_tree().change_scene_to_file(scene_path)
	if err != OK:
		push_error("[SceneManager] 切换场景失败: %s (err=%d)" % [scene_path, err])
		return
	_stack.append(scene_path)
	_current = scene_path
	scene_changed.emit(scene_path)


## 返回上一场景。栈深度 ≤ 1 时返回 false。
func pop() -> bool:
	if _stack.size() <= 1:
		return false
	_stack.pop_back()
	var prev: String = _stack[_stack.size() - 1]
	var err := get_tree().change_scene_to_file(prev)
	if err != OK:
		push_error("[SceneManager] 返回场景失败: %s (err=%d)" % [prev, err])
		return false
	_current = prev
	scene_changed.emit(prev)
	return true


## 清空栈并跳转到指定场景
func reset_to(scene_path: String) -> void:
	var err := get_tree().change_scene_to_file(scene_path)
	if err != OK:
		push_error("[SceneManager] 重置场景失败: %s (err=%d)" % [scene_path, err])
		return
	_stack.clear()
	_stack.append(scene_path)
	_current = scene_path
	scene_changed.emit(scene_path)
