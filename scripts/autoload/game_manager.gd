extends Node

## GameManager — 游戏全局状态机(单例)
## 管理游戏全局状态流转,对应核心循环: 发现 → 捕获 → 收藏(→ 战斗)。
## 参考: 游戏开发计划.md 3.1 核心循环、4.1 客户端架构分层。
##
## 注意: 本单例只管"游戏状态机"层面的状态;网络在线状态归 NetworkManager,
## 存档读写归 SaveManager,场景导航归 SceneManager。各司其职。

signal state_changed(from_state: int, to_state: int)

enum GameState {
	BOOT,        # 启动初始化
	MAIN_MENU,   # 主菜单
	DISCOVER,    # 发现 (LBS 探索 + 云端 VLM 检测)
	CAPTURE,     # 捕获 (3D 物理投掷小游戏)
	COLLECT,     # 收藏 (Pet-dex 图鉴浏览)
	BATTLE,      # 战斗 (内测阶段起, MVP 占位)
	SETTINGS,    # 设置
}

var _current_state: int = GameState.BOOT
var _previous_state: int = GameState.BOOT


func _ready() -> void:
	# TODO(M14): 启动时校验隐私授权, 拒绝则降级"仅浏览"模式
	# TODO(M12): 读取玩家等级/进度
	# MVP 阶段直接进入主菜单
	transition_to(GameState.MAIN_MENU)


## 当前游戏状态
func current_state() -> int:
	return _current_state


## 上一个游戏状态(用于返回)
func previous_state() -> int:
	return _previous_state


## 状态名称(用于日志/调试)
func state_name(s: int) -> String:
	return GameState.keys()[s]


## 切换状态。非法切换会被拒绝并打印警告。
## 返回是否切换成功。
func transition_to(new_state: int) -> bool:
	if new_state == _current_state:
		return true
	if not _is_transition_valid(_current_state, new_state):
		push_warning("[GameManager] 非法状态切换: %s -> %s" % [state_name(_current_state), state_name(new_state)])
		return false
	_previous_state = _current_state
	_current_state = new_state
	state_changed.emit(_previous_state, _current_state)
	print("[GameManager] %s -> %s" % [state_name(_previous_state), state_name(_current_state)])
	return true


## 校验状态机切换合法性。
## 在线优先(4.3): DISCOVER/CAPTURE 需联网, 断网时拦截。
## MVP 不强约束业务态互转, 内测起按业务补充。
func _is_transition_valid(from: int, to: int) -> bool:
	# 断网时禁止进入需要联网的状态
	if to == GameState.DISCOVER or to == GameState.CAPTURE:
		# 用运行时节点查找引用 NetworkManager(autoload), 不直接写全局名。
		# 原因: Godot 在外部修改 project.godot 后, 若编辑器未重新加载项目,
		# autoload 全局符号尚未注册到 GDScript 编译器, 直接写 NetworkManager.xxx()
		# 会触发 Parse Error "Identifier not declared in the current scope"。
		# autoload 单例在场景树路径为 /root/<SingletonName>。
		var nm: Node = get_node_or_null("/root/NetworkManager")
		if nm != null and nm.has_method("is_offline") and nm.call("is_offline"):
			push_warning("[GameManager] 断网状态, 发现/捕获不可用 (在线优先架构 4.3)")
			return false

	# 体力不足时禁止捕获 (M1 体力系统)
	if to == GameState.CAPTURE:
		var stm: Node = get_node_or_null("/root/StaminaManager")
		if stm != null and stm.has_method("can_capture") and not stm.call("can_capture"):
			push_warning("[GameManager] 体力不足, 无法捕获 (体力系统 3.4)")
			return false

	return true
