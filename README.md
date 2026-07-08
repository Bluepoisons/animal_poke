# animal_poke

LBS 动物收集手游（基于 CatchCat 概念改进）。真实世界探索 + 云端 VLM 实时动物识别 + 云端 LLM 数值生成。

> 设计文档单一事实来源：[`游戏开发计划.md`](游戏开发计划.md) v1.2（2026-07-08）
> 执行层任务清单：[`项目开发任务清单.md`](项目开发任务清单.md)

---

## 技术栈

| 层 | 选型 |
|----|------|
| 游戏引擎 | Godot 4.7（Forward Plus 渲染器） |
| 物理引擎 | Jolt Physics 3D（投掷小游戏） |
| UI 框架 | Godot Control + Theme 主题系统 |
| AR 层 | GDExtension 绑定 ARKit(iOS)/ARCore(Android) |
| 云端 AI | 云端 VLM（视觉）+ 云端 LLM（数值/叙事），客户端零本地推理 |
| 架构模式 | 在线优先（发现/捕获需联网，断网仅图鉴浏览） |
| 本地存储 | SQLite + 加密 |
| API Key | 统一 `.env`（已 gitignore），环境变量读取，禁止硬编码 |

---

## 目录结构

```
animal_poke/
├── project.godot              # 引擎配置 + Autoload 注册
├── icon.svg                   # 应用图标
├── 游戏开发计划.md             # 设计文档（唯一事实来源，勿在此散落过程文件）
├── 项目开发任务清单.md          # 执行层任务清单
├── README.md                  # 本文件（目录约定 + 团队规范）
│
├── scenes/                    # Godot 场景文件 (.tscn)
│   └── （MVP M13 落地: discover/ capture/ collect/ onboarding/ main.tscn）
│
├── scripts/
│   ├── autoload/              # 全局 Autoload 单例（见下表）
│   ├── core/                  # 核心层脚本（非业务模块）
│   │   └── （config_loader / db / logger / ai / security / sync，对应 F3-F5、M1-M2）
│   ├── modules/               # 业务模块脚本（对应核心循环各阶段）
│   │   └── （discover/ capture/ collect/ stamina/ economy/ progress，对应 M3-M12）
│   └── ui/                    # UI 层脚本
│       ├── rarity.gd          # 稀有度枚举 + 边框色（灰/绿/蓝/紫/金，F2）
│       └── components/        # 基础 UI 组件（按钮/面板/稀有度边框/loading/Toast，F2）
│
├── resources/                 # Godot 资源 (.tres/.json)：稀有度配置、物种属性表、道具表等
├── assets/                    # 原始美术/音频资源（纹理/模型/音效/字体）
└── themes/                    # Godot Theme 资源（default_theme.tres，F2 全局应用）
```

> 空目录用 `.gdkeep` 占位以纳入版本控制，业务内容随对应任务填充。

---

## Autoload 全局单例

在 `project.godot` `[autoload]` 段注册。**按依赖顺序初始化**（基础单例在前，GameManager 最后）。任意场景可直接以单例名访问，如 `NetworkManager.is_online()`。

| 单例 | 脚本 | 职责 | 初始化顺序 | 对应任务 |
|------|------|------|-----------|---------|
| `ConfigManager` | `scripts/autoload/config_manager.gd` | 客户端配置读取(BACKEND_BASE_URL + 设备 Token);不含第三方 Key | 1 | F1 骨架 / F3 完善 |
| `Logger` | `scripts/core/logger.gd` | 分级日志(DEBUG/INFO/WARN/ERROR) + 崩溃落盘 | 2 | F5 |
| `NetworkManager` | `scripts/autoload/network_manager.gd` | 网络在线状态（ONLINE/WEAK/OFFLINE） | 3 | F1 骨架 / M5 完善 |
| `SaveManager` | `scripts/autoload/save_manager.gd` | 本地存档读写 | 3 | F1 骨架 / F4 接 SQLite |
| `AudioManager` | `scripts/autoload/audio_manager.gd` | 音效/BGM 播放与总线音量 | 4 | F1 骨架 / F2 补资源 |
| `SceneManager` | `scripts/autoload/scene_manager.gd` | 场景切换栈（push/pop/replace） | 5 | F1 骨架 / M13 用 |
| `GameManager` | `scripts/autoload/game_manager.gd` | 游戏状态机（BOOT/MAIN_MENU/DISCOVER/CAPTURE/COLLECT/BATTLE） | 6 | F1 骨架 / 后续完善 |

**依赖说明**：`GameManager._ready()` 会调用 `NetworkManager.is_offline()` 做断网拦截（在线优先架构 4.3），故 `NetworkManager` 必须先于 `GameManager` 初始化。调整 autoload 顺序时注意保持此约束。

> autoload 脚本之间互引用时，用 `/root/<SingletonName>` 路径运行时查找（规避编译期符号注册时序问题，见 F1 完成记录）；普通场景脚本仍可直接用全局名。

---

## UI 主题与基础组件（F2）

- **全局 Theme**：`themes/default_theme.tres`，已通过 `project.godot [gui] theme/custom` 全局应用。定义按钮/面板/Label 默认样式（**暖色橙色调卡通风**：主色 `#FF8C42` / 奶油背景 `#FFF8F0` / 深棕文字 `#4A2C1A`，按钮带 4px 实色下沿模拟立体按压感）。外部改 `project.godot` 后需在编辑器「项目 → 重新加载当前项目」生效。
- **稀有度颜色**：`scripts/ui/rarity.gd`（`class_name Rarity`），边框色与 5.1 表一致——灰/绿/蓝/紫/金，传说级带粒子特效位。用 `Rarity.color_of(Rarity.Tier.RARE)` 取色。
- **基础 UI 组件**（`scripts/ui/components/`，均纯代码构建，无 `.tscn` 依赖）：

| 组件 | class_name | 说明 |
|------|-----------|------|
| 稀有度边框 | `RarityBorder` | `set_rarity(tier)` 显示对应色边框，叠加在卡片上 |
| 基础面板 | `BasePanel` | 继承 Theme 面板样式，`set_title()` 可加标题 |
| 基础按钮 | `AppButton` | 带 0.2s 防抖 + 点击音效占位，连 `clicked` 信号 |
| 加载提示 | `LoadingIndicator` | 旋转弧线动画 + 可选文字 |
| Toast 提示 | `Toast` | `Toast.popup("消息")` 顶层浮层，自动淡入淡出销毁 |

---

## 团队开发规范

### 文档约定
- **设计变更只写入 `游戏开发计划.md`**，不产出散落的过程文件（对辩汇总、评估图等）。
- **任务推进更新 `项目开发任务清单.md`** 状态（`[ ]` → `[x]`）。
- 这两个文件是唯一事实来源，其余文档随对应任务产生。

### API Key 管理（客户端零第三方 Key）
- **第三方 Key（腾讯地图/彩云天气/云端 VLM/LLM）只在 Go 后端 `.env`（见 `backend/.env.example`），客户端永不含第三方 Key。**
- 客户端 `.env`（根目录 `.env.example`）**只存非敏感配置**：`BACKEND_BASE_URL` / `LOG_LEVEL` / `DB_FILENAME`。
- 客户端登录后保存后端下发的**设备 Token**，存于 `user://auth/device_token.dat`（不进 `.env`）。
- 所有 `.env` 均已加入 `.gitignore`，**切勿硬编码进代码或提交 git**。
- 客户端配置读取：`ConfigManager.get_backend_base_url()` 等，底层走 `ConfigLoader`（优先级：OS 环境变量 > `res://.env` > 默认值）。

### Git 规范
- 用 **rebase** 保持线性历史：`git pull --rebase`。
- 不在提交信息中暴露任何 key 或敏感信息。

### Godot 开发
- 引擎版本：**Godot 4.7**。新成员请先安装对应版本。
- 场景文件（`.tscn`）与脚本（`.gd`）分开存放：场景在 `scenes/`，逻辑在 `scripts/`。
- 新建业务模块时在 `scripts/modules/<module>/` 下组织，对应 MVP 任务编号。
- 空目录放 `.gdkeep` 占位，避免丢失目录结构。

### 性能与质量基线（硬性，见任务清单第九节）
- 中端机（骁龙 6 系 / A12）≥ 30fps，高端机 ≥ 60fps。
- 安装包 ≤ 150MB（无端侧模型）。
- 崩溃率 < 0.5%。
- 图鉴列表必须虚拟化渲染（防 CatchCat 滚动卡顿）。
- 在线优先：发现/捕获/数值生成必须联网；断网仅图鉴浏览，有明确提示。

---

## 运行项目

### 环境要求

- **Godot 4.7**（与 `project.godot` `config/features` 一致，低版本不兼容）
- 安装方式：
  - **macOS**：`brew install --cask godot`（装好后命令为 `godot`，App 在 `/Applications/Godot.app`）
  - **Windows / Linux**：从 [godotengine.org](https://godotengine.org/download) 下载 4.7 稳定版，建议把可执行文件加入 `PATH`

### 方式一：编辑器运行（推荐开发调试）

1. 打开 Godot 4.7。
2. 项目管理器 → **导入** → 选择本目录下的 `project.godot`。
3. 首次打开会自动导入资源并编译脚本（状态栏可见进度）。
4. 按 **F5** 运行项目。主场景已设为 `res://scenes/main.tscn`，启动后进入主界面（顶部状态栏 + 底部三 Tab：发现 / 图鉴 / 我的），可在三屏间切换。
5. Autoload 单例会在启动时打印初始化日志，可在 **Output** 面板确认（ConfigManager / NetworkManager / SaveManager / AudioManager / GameManager 依次初始化）。

### 方式二：命令行运行

```bash
# 替换 <项目路径> 为本目录的绝对路径
godot --path <项目路径>
```

### 运行测试（headless，无需 GUI）

项目在 `tests/` 下提供 GDScript 测试套件（`extends SceneTree`，用 `godot --headless -s` 运行）。当前覆盖三个 M1 模块：

```bash
# 体力系统（M10，80 项断言）
godot --headless --path <项目路径> -s tests/test_stamina.gd

# 基础 UI 三屏（M13，41 项断言）
godot --headless --path <项目路径> -s tests/test_ui.gd

# 金币 + 道具商店（M11，107 项断言）
godot --headless --path <项目路径> -s tests/test_economy.gd
```

退出码 `0` = 全部通过，非 `0` = 有失败。输出末尾会打印 `通过: N  失败: M` 汇总。

> **测试编写约定**：纯数值逻辑抽成 `RefCounted` 类（如 `StaminaModel` / `WalletModel`），Node 层只管 IO/信号/持久化，便于 headless 单测不依赖场景树。测试脚本里引用新 `class_name` 时，若 headless `-s` 报 "Identifier not declared"（全局 class 缓存未刷新），改用 `const X = preload("...")` 直接拿脚本资源绕开。

### 当前可运行内容

- **主场景** `scenes/main.tscn`：三屏（发现 / 捕获 / 图鉴）+ 顶部状态栏 + 底部 Tab 切换，暖色橙卡通主题。
- **发现屏**：取景框占位 + 检测框 + 城市天气提示 + 扫描按钮（真实相机/VLM 待 M3）。
- **捕获屏**：投掷场景占位 + 力度条 + 抛物线预览 + 投掷按钮（真实 3D 物理投掷待 M6）。
- **图鉴屏**：卡片网格 + 物种筛选 Tab + 详情弹窗（六维属性占位）。
- **体力 / 经济模块**：纯逻辑已实现并通过测试，尚未挂载到场景（集成时由主场景 `add_child` 或 GameManager 持有）。状态栏体力/金币数据当前为 mock，待接入真实 manager。
- **后端**：Go 后端脚手架待 F6，联网功能（发现/捕获/同步）未接通，当前为离线可运行的客户端骨架。
### 客户端（Godot）
1. 安装 Godot 4.7。
2. 用 Godot 项目管理器导入本目录。
3. 安装 **godot-sqlite 插件**（`addons/godot-sqlite`），并在「项目 → 项目设置 → 插件」中启用（F4 本地数据库依赖，未安装时 `LocalDB.open()` 会报错降级）。
4. 复制 `.env.example` 为 `.env` 填入 `BACKEND_BASE_URL`。
5. 运行项目（F5）。**注意**：MVP 主场景待 Task M13 落地，当前运行会提示"未设置主场景"——这是预期行为，非报错。Autoload 单例（含新增 `Logger`）会在启动时打印初始化日志，可在 Output 面板确认。

### 后端（Go，见 `backend/`）
```bash
cd backend
make db-up     # 本地 Docker 起 MySQL 开发服务器
cp .env.example .env   # 填入第三方 Key
make run       # 启动服务, /health 返回 200
```
详见 [`backend/README.md`](backend/README.md)。

> Foundation 阶段（F1-F6）目标：搭好工程地基。客户端配置/数据库/日志骨架就位；Go 后端作为联网服务总枢纽，承载全部第三方 Key 与 `/health`、中间件链，为 MVP（M1-M14、MB1-MB5）铺路。
