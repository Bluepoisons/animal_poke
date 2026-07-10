# Godot Foundation 历史完成记录（Archive）

> **状态：已退役**。以下内容来自 2026-07-08 前后 Godot 实现阶段的完成记录，
> 仅作考古用途。**当前交付以 React + Go 为准**，可执行 backlog 见
> `docs/grok-issue-backlog-2026-07-10.md` 与 GitHub Issues。
>
> 源文件：`docs/项目开发任务清单.md`（AP-060 归档）。

---

**完成记录**(2026-07-08,Godot 实现,已于 v1.4 前端迁移至 React 后退役;功能在 React 前端重新实现):
- 交付物: 8 个目录(scripts/{autoload,core,modules} + scenes/resources/assets/themes,空目录用 .gdkeep 占位);6 个 autoload 脚本;project.godot [autoload] 注册;README 目录约定 + 团队规范。
- Autoload 初始化顺序: ConfigManager → NetworkManager → SaveManager → AudioManager → SceneManager → GameManager(按依赖排列,GameManager 依赖 NetworkManager 做在线优先断网拦截 4.3)。
- 验收: ①目录结构/团队约定写入 README ✅;②6 个 autoload 在 project.godot 注册(带 * 启用),任意场景可访问 ✅;③项目可启动无报错 — 初次实跑触发 GameManager Parse Error(autoload 跨引用,编译期全局符号未注册),已修复(改用 `get_node_or_null("/root/NetworkManager")` 运行时查找),用户重新加载项目后确认通过。
- 关键决策: autoload 脚本之间互引用 `/root/<SingletonName>` 路径运行时查找(规避编译期符号注册时序问题);普通场景脚本仍可直接用全局名。各单例为可运行骨架,关键实现用 TODO 指向 F3-F5/M5/M13,不越界后续任务。

---

**完成记录**(2026-07-08,Godot 实现,已于 v1.4 前端迁移至 React 后退役;功能在 React 前端重新实现):
- 交付物: themes/default_theme.tres(全局 Theme: 按钮 normal/hover/pressed/disabled 样式 + 面板样式 + Label 颜色,深色面板+蓝绿强调色 #2EA88A);scripts/ui/rarity.gd(class_name Rarity,稀有度枚举+边框色);5 个 UI 组件(scripts/ui/components/)。
- 稀有度边框色严格对齐 5.1 表: 灰(0.55,0.55,0.55)/绿(0.30,0.80,0.30)/蓝(0.30,0.55,1.00)/紫(0.65,0.35,0.95)/金(1.00,0.80,0.20),传说级预留粒子特效位。
- 全局应用: project.godot 新增 [gui] theme/custom="res://themes/default_theme.tres"。外部改 project.godot 后需在编辑器「项目→重新加载当前项目」。
- 组件均为纯代码构建(无 .tscn 依赖),用 class_name 注册: RarityBorder/BasePanel/AppButton(0.2s 防抖+点击音效占位)/LoadingIndicator(旋转弧线动画)/Toast(顶层浮层 Toast.popup() 自动淡入淡出)。
- 验收: ①Theme 全局应用 ✅(project.godot 注册);②稀有度边框色与 5.1 表一致 ✅;③基础组件可复用 ✅(class_name + 纯代码构建)。待用户重新加载项目实跑确认。

---

**完成记录**(2026-07-08,Godot 实现,已于 v1.4 前端迁移至 React 后退役;功能在 React 前端重新实现):
- 新增 `scripts/core/config_loader.gd`(`class_name ConfigLoader extends RefCounted`):健壮 .env 解析(注释/成对引号/类型转换),`get_string/get_int/get_float/get_bool`,优先级 OS 环境变量 > `res://.env` > 默认值。
- 改写 `scripts/autoload/config_manager.gd`:移除原第三方 Key 常量,改用 `ConfigLoader`;提供 `get_backend_base_url()` / `get_log_level()` / `get_db_filename()`;设备 Token 存 `user://auth/device_token.txt`(`set/get/clear_device_token`),不进 `.env`。
- 根 `.env.example`:仅 `BACKEND_BASE_URL` / `LOG_LEVEL` / `DB_FILENAME`,无第三方 Key。
- 修正根 `.gitignore`:`.env.*` 之后加 `!.env.example`,确保 `.env.example` 被追踪、`.env` 仍忽略(`git check-ignore` 验证通过)。

---

**完成记录**(2026-07-08,Godot 实现,已于 v1.4 前端迁移至 React 后退役;功能在 React 前端重新实现):
- 新增 `scripts/core/schema.sql`:四表 `animals`/`player_progress`(单行 id=1)/`inventory`/`checkin`;`animals` 含 UUID/稀有度/六维属性(HP/ATK/DEF/SPD/Class/Element)/品种/物种/坐标(lat,lng)/生成时间/推理请求ID/模型版本/捕获方式,全部 `CREATE TABLE IF NOT EXISTS`。
- 新增 `scripts/core/db.gd`(`class_name LocalDB extends RefCounted`):封装 godot-sqlite 插件 CRUD(`insert_rows`/`select_rows`/`update_rows`/`delete_rows`,条件为字符串、`_sql_escape` 转义单引号防注入);`open()` 时执行 schema.sql(按 `;` 拆分)、设置 `encryption_key`(SQLCipher,密钥首次生成存 `user://.dbkey`,不进 `.env`);未安装插件时 `open()` 报错并返回 false。
- 改写 `scripts/autoload/save_manager.gd`:`_ready()` 中通过 `LocalDB` 建库并接管动物/进度/背包/签到持久化;`_notification(NOTIFICATION_WM_CLOSE_REQUEST)` 时 `close()`,满足"重启不丢数据"。
- 注:godot-sqlite 插件需手动安装(`addons/godot-sqlite` + 插件管理器启用),README 已补充说明。

---

**完成记录**(2026-07-08,Godot 实现,已于 v1.4 前端迁移至 React 后退役;功能在 React 前端重新实现):
- 新增 `scripts/core/logger.gd`(`class_name Logger extends Node`):`DEBUG/INFO/WARN/ERROR` 分级,写 `user://logs/app.log` + 内存环形缓冲(RING_MAX=500);`LOG_LEVEL` 可由 `ConfigManager` 注入。
- 退出/崩溃落盘:正常退出(`NOTIFICATION_WM_CLOSE_REQUEST`/`NOTIFICATION_PREDELETE`)仅刷新文件缓冲;`report_crash()` 显式触发时将内存环形缓冲落盘到 `user://logs/crash_last.log`;`flush_crash()` 预留 Bugly/自建上报入口(TODO)。注:硬崩溃(段错误/kill)GDScript 无法捕获,需后续接 OS 级信号处理器(F5"后续接 Bugly/自建")。
- `project.godot` `[autoload]` 注册 `Logger`(置于 `ConfigManager` 之后、`NetworkManager` 之前),全局可直接 `Logger.info(...)` 调用。

---

**完成记录**(2026-07-08, 持久化部分,Godot 实现,已于 v1.4 前端迁移至 React 后退役;功能在 React 前端重新实现):
- 新增 `scripts/modules/collect/animal_repository.gd`(`class_name AnimalRepository extends RefCounted`): 动物元数据领域层。职责: ① RFC4122 v4 UUID 生成(1000 次无碰撞); ② ISO8601 UTC 时间戳自动填充(`created_at`/`updated_at`/`generated_at`); ③ 数据校验(species 非空 / rarity 0-4 / 六维属性范围符合 5.2, 0 视为"待 M2 云端生成"跳过校验); ④ **隐私强制(4.5)**: upsert 前剥离 `photo`/`image`/`raw_frame`/`thumbnail` 等键, 防止原始照片落盘本地; ⑤ CRUD 委托 `LocalDB`(构造注入, 生产 `SaveManager.get_db()`, 测试直接注入)。
- `scripts/autoload/save_manager.gd` 新增 `get_db() -> LocalDB` 公开访问器, 供业务模块构造 repository。
- **验收 1(UUID 唯一, 重启不丢)✅**: `tests/integration/animal_persistence.test.gd` 4 个用例直接验证 issue #2 验收标准 —— 捕获→关闭→重启→数据仍在(uuid/species/rarity/属性/坐标/推理ID 一致); 多会话累积; 跨会话 UUID 唯一(3 会话×5 只=15 个全唯一); photo 跨重启不落盘。
- **验收 3(原始照片不落盘)✅**: repository 层 `_strip_photo_keys` 在 upsert 前剥离所有 photo 相关键, 集成测试 `test_photo_not_persisted_across_restart` 验证重启后仍不存在。
- **验收 2(同步成功/失败可重试)⏸**: 依赖后端 MB4(`/sync/animal` 端点 + 反作弊审计), 后端 MVP 任务 MB4 未启动。`sync_service.gd` 待 MB4 完成后实现。
- 测试: 单元 `tests/unit/animal_repository.test.gd`(15 用例: UUID 格式/唯一性/校验/隐私剥离/时间戳/CRUD/guard)+ 集成 `tests/integration/animal_persistence.test.gd`(4 用例)。全套 102 用例 0 失败。
- **issue #2 [M1] 本地存储**: 验收标准"动物数据可持久化,重启不丢失"已通过集成测试验证, 可关闭。

#### [ ] Task M9: Pet-dex 图鉴 UI
**Description**:图鉴列表(**虚拟化渲染防卡顿**)+ 详情页(六维属性/稀有度边框/品种/生成依据明细)。离线可浏览。
**Acceptance Criteria**:
- 列表滚动无卡顿(虚拟化)
- 详情可查看生成依据四项得分
- 离线可浏览已缓存数据

**Files**: `frontend/src/components/CollectScreen.tsx`、`DetailPopup.tsx`、`frontend/src/modules/collect/*.ts`
**Reference**: 6.2 图鉴模块、5.3 透明性、九-UI卡顿风险(虚拟化)

---

**完成记录**(2026-07-08,Godot 实现,已于 v1.4 前端迁移至 React 后退役;功能在 React 前端重新实现):
- 交付物: `scripts/modules/stamina/stamina_manager.gd`(autoload 单例),注册于 project.godot SaveManager 之后。
- 核心实现: 自然恢复(每 6 分钟 +1,离线时间差计算)、捕获/派遣消耗(各 20 体力)、升级回满(on_level_up 更新上限表)、体力购买(purchase_stamina 上限封顶)、体力为零时 GameManager 拦截 CAPTURE 状态切换。
- 体力上限查表法: Lv.1-9 = 120 + 14×(level-1), Lv.10 = 240(增量从 +14 降为 +8,对齐 5.5 设计表)。
- 信号: stamina_changed(消耗/恢复/升级/购买)、stamina_insufficient(体力不足提示)。
- 持久化: 通过 SaveManager 读写 player_progress 表的 stamina/stamina_updated_at 字段。
- 测试: `tests/unit/stamina_manager.test.gd`(34 用例: 默认值/恢复数学/消耗/升级/查表/信号/多次消耗) + `tests/integration/stamina_system.test.gd`(8 用例: DB 重启不丢/autoload 链路/SaveManager 读写/时间戳更新/空库初始化)。42 个新增用例全部通过,0 失败。
- **验收 1(自然恢复计时准确)✅**: 单元测试覆盖 0/1/N 档恢复、上限封顶、时间戳更新;集成测试验证离线时间差计算与 DB 持久化。
- **验收 2(升级满体力)✅**: on_level_up 更新 max + restore_to_full,单元测试覆盖 Lv.1→Lv.2/Lv.3。
- **验收 3(购买限购生效)⏸**: purchase_stamina 方法已就位(上限封顶逻辑),每日限购 3 次需 M11 商店系统接入。
- **验收 4(体力 0 时捕获入口禁用)✅**: GameManager._is_transition_valid 中追加体力检查,体力不足时拒绝进入 CAPTURE 状态;autoload_boot 集成测试验证拦截行为。
- 依赖: M11 商店(体力药剂购买)、M12 等级系统(升级调 on_level_up)。StaminaManager 已为二者暴露 API。
- **issue #1 [M1] 体力系统**: 验收标准"每小时恢复 10 点,单次捕获消耗 20,上限 120→240(满级),升级恢复满体力"已通过测试验证,可关闭。

#### [ ] Task M11: 经济系统(金币 + 商店 + 签到)
**Description**:金币软货币 + 道具商店:玩具球(+15% 成功率,50 金币)、食物包(补 10 投掷物,30 金币)、体力药剂(150,日限购 3)。每日签到连续递增(第 1 天 20 → 第 7 天 150 + 道具,断签重置)。
**Acceptance Criteria**:
- 金币产出/消耗闭环
- 商店可购买使用
- 签到断签重置生效

**Files**: `frontend/src/modules/economy/wallet.ts`、`shop.ts`、`checkin.ts`
**Reference**: 6.4 经济系统

> **PM 注**:6.4 的"每日商店随机刷新"依赖等级加成(5.5),MVP 不做,放内测。双货币的硬货币(MVP 暂只金币)放公测商业化阶段。

#### [ ] Task M12: 等级系统(MVP 简化版)
**Description**:玩家等级与累计捕获数挂钩(Lv.1→Lv.2 需 10 只)。升级恢复满体力 + 金币回馈。MVP 只做 Lv.1-3 的基础升级,完整 10 级表放内测。
**Acceptance Criteria**:
- 累计捕获达标自动升级
- 升级恢复满体力 + 发金币
- 等级 UI 显示

**Files**: `frontend/src/modules/progress/levelManager.ts`
**Reference**: 5.5 等级表(MVP 取前 3 级)

---

