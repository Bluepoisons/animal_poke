extends Node

## SaveManager — 存档管理(单例)
## 负责本地存档读写。F1 阶段提供接口骨架, 具体 SQLite + 加密实现见 F4。
## 参考: 游戏开发计划.md 4.4 同步方案、4.1 本地数据库、4.5 隐私(原始照片不落盘)。
##
## 同步内容(4.4): UUID / 稀有度 / 属性 / 品种 / 物种 / 捕获坐标(可选脱敏) /
## 生成时间 / 云端推理请求 ID。schema 见 scripts/core/schema.sql (F4)。

signal saved(slot: int)
signal loaded(slot: int)

const DEFAULT_SLOT := 0

var _initialized: bool = false


func _ready() -> void:
	# TODO(F4): 初始化 SQLite 连接 + 加密, 执行 schema.sql 建表
	# 表: 动物表 / 玩家进度表 / 背包表 / 签到表
	_initialized = true
	print("[SaveManager] 初始化完成 (骨架, SQLite 待 F4 接入)")


## 是否已完成初始化(供其他 Manager 判断是否可存取)
func is_ready() -> bool:
	return _initialized


## 保存到指定存档槽(默认 0)。返回是否成功。
func save_game(slot: int = DEFAULT_SLOT) -> bool:
	# TODO(F4): 序列化玩家进度/动物元数据/背包/签到 写入 SQLite
	push_warning("[SaveManager] save_game 尚未实现 (待 F4)")
	return false


## 从指定存档槽加载。返回是否成功。
func load_game(slot: int = DEFAULT_SLOT) -> bool:
	# TODO(F4): 从 SQLite 读取并反序列化
	push_warning("[SaveManager] load_game 尚未实现 (待 F4)")
	return false


## 存档是否存在
func has_save(slot: int = DEFAULT_SLOT) -> bool:
	# TODO(F4): 查询 SQLite
	return false
