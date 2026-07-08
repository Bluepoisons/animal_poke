# animal_poke

LBS 动物收集手游（基于 CatchCat 概念改进）。真实世界探索 + 云端 VLM 实时动物识别 + 云端 LLM 数值生成。

> 设计文档单一事实来源：[`游戏开发计划.md`](游戏开发计划.md) v1.4（2026-07-08，前端 Godot→React）
> 执行层任务清单：[`项目开发任务清单.md`](项目开发任务清单.md)

---

## 技术栈

| 层 | 选型 |
|----|------|
| 前端框架 | React 18 + Vite 6 + TypeScript 5.6（Web/PWA） |
| 物理/捕获 | Canvas 2D（MVP）/ react-three-fiber（内测，3D 物理投掷） |
| UI 框架 | React 组件 + CSS 变量主题（暖橙卡通风格） |
| 相机/定位 | 浏览器 getUserMedia + 系统定位（Geolocation） |
| 云端 AI | 云端 VLM（视觉）+ 云端 LLM（数值/叙事），客户端零本地推理 |
| 后端服务 | Go（Gin + Gorm）+ API 网关（联网服务总枢纽） |
| 架构模式 | 在线优先（发现/捕获需联网，断网仅图鉴浏览） |
| 本地存储 | 浏览器 IndexedDB + 加密（localStorage 轻量） |
| API Key | 统一 `.env`（已 gitignore），环境变量读取，禁止硬编码 |

---

## 目录结构

```
animal_poke/
├── frontend/                 # React 前端（Vite + TypeScript）
│   ├── index.html            # 入口 HTML
│   ├── package.json          # 依赖与脚本（dev/build/preview）
│   ├── vite.config.ts        # Vite 配置（/api 代理到 Go 后端 :8080）
│   ├── tsconfig.json
│   └── src/
│       ├── main.tsx          # React 挂载入口
│       ├── App.tsx           # 顶层布局：TopBar + 内容切换 + TabBar
│       ├── index.css         # 全局 CSS 变量主题（暖橙卡通风格）
│       ├── types.ts          # 类型定义 + mock 数据
│       ├── context/          # 全局状态层（见下表）
│       ├── core/             # 核心层（config / db / logger / ai / security / sync）
│       ├── modules/          # 业务模块（discover/ capture/ collect/ stamina/ economy/ progress）
│       └── components/       # UI 组件（TopBar/ TabBar/ CollectScreen/ MapScreen/ DiscoverScreen/ ...）
│
├── backend/                  # Go 后端（联网服务总枢纽，见 backend/README.md）
│   ├── cmd/main.go
│   ├── internal/{config,routes,handlers,services,middleware,repo}
│   └── go.mod
│
├── 游戏开发计划.md             # 设计文档（唯一事实来源，勿在此散落过程文件）
├── 项目开发任务清单.md          # 执行层任务清单
├── README.md                  # 本文件（目录约定 + 团队规范）
└── icon.svg                   # 应用图标
```

---

## 全局状态层（React Context / 轻量 store，对应 Foundation F1）

前端通过 React Context Provider 实现全局单例状态，在 `App.tsx` 顶层包裹，任意组件可消费：

| Provider | 文件 | 职责 | 对应任务 |
|----------|------|------|---------|
| `ConfigProvider` | `src/context/config.tsx` | 客户端配置读取（BACKEND_BASE_URL + 设备 Token）；不含第三方 Key | F1 骨架 / F3 完善 |
| `LoggerProvider` | `src/context/logger.tsx` | 分级日志（DEBUG/INFO/WARN/ERROR） | F5 |
| `NetworkProvider` | `src/context/network.tsx` | 网络在线状态（ONLINE/WEAK/OFFLINE） | F1 骨架 / M5 完善 |
| `SaveProvider` | `src/context/save.tsx` | 本地存档读写（IndexedDB） | F1 骨架 / F4 接 IndexedDB |
| `GameProvider` | `src/context/game.tsx` | 游戏状态机（BOOT/MAIN/DISCOVER/CAPTURE/COLLECT/...） | F1 骨架 / 后续完善 |

> 依赖说明：`GameProvider` 在切换捕获状态时调用 `NetworkProvider` 做断网拦截（在线优先架构 4.3），故 `NetworkProvider` 必须先于 `GameProvider` 挂载。

---

## UI 主题与基础组件（F2）

- **全局主题**：`src/index.css` 定义 CSS 变量（**暖色橙色调卡通风**：主色 `#FF8C42` / 深橙 `#E67300` / 奶油背景 `#FFF8F0` / 深棕文字 `#4A2C1A`，体力黄 `#FFD23F` / 金币金 `#FFB300`；按钮带 4px 实色下沿模拟立体按压感）。组件统一引用变量，禁止硬编码颜色。
- **稀有度颜色**：边框色与 5.1 表一致——灰/绿/蓝/紫/金（见 `src/types.ts` 或 `src/components/ui/` 中的 `Rarity` 常量），传说级带粒子特效位。用 `RARITY[tier].color` 取色。
- **基础 UI 组件**（`src/components/ui/`）：稀有度边框 `RarityBorder`、基础面板 `BasePanel`、基础按钮 `AppButton`（带防抖 + 点击反馈）、加载提示 `LoadingIndicator`、Toast 提示 `Toast`。

---

## 团队开发规范

### 文档约定
- **设计变更只写入 `游戏开发计划.md`**，不产出散落的过程文件（对辩汇总、评估图等）。
- **任务推进更新 `项目开发任务清单.md`** 状态（`[ ]` → `[x]`）。
- 这两个文件是唯一事实来源，其余文档随对应任务产生。

### API Key 管理（客户端零第三方 Key）
- **第三方 Key（腾讯地图/彩云天气/云端 VLM/LLM）只在 Go 后端 `.env`（见 `backend/.env.example`），客户端永不含第三方 Key。**
- 客户端 `.env`（`frontend/.env`，根目录 `.env.example` 提供参考）**只存非敏感配置**：`BACKEND_BASE_URL` / `LOG_LEVEL`。
- 客户端登录后保存后端下发的**设备 Token**，存于 `localStorage`（不进 `.env`）。
- 所有 `.env` 均已加入 `.gitignore`，**切勿硬编码进代码或提交 git**。
- 客户端配置读取：通过 `ConfigProvider` 暴露 `getBackendBaseUrl()` 等。

### Git 规范
- 用 **rebase** 保持线性历史：`git pull --rebase`。
- 不在提交信息中暴露任何 key 或敏感信息。

### 前端（React）开发
- 包管理：npm（或 pnpm）。新成员先 `npm install`。
- 业务模块组织在 `src/modules/<module>/` 下，对应 MVP 任务编号。
- 全局状态用 `src/context/` 下的 Provider，不要在组件里散落单例。

### 性能与质量基线（硬性，见任务清单第九节）
- 中端机（骁龙 6 系 / A12）≥ 30fps，高端机 ≥ 60fps。
- 安装包 ≤ 150MB（无端侧模型；Web/PWA 按需加载）。
- 崩溃率 < 0.5%。
- 图鉴列表必须虚拟化渲染（防 CatchCat 滚动卡顿）。
- 在线优先：发现/捕获/数值生成必须联网；断网仅图鉴浏览，有明确提示。

---

## 运行项目

### 环境要求
- **Node.js 18+**（推荐 20+），npm 9+。
- **Go 1.22+**（后端）。

### 前端（React）
```bash
cd frontend
npm install
cp .env.example .env   # 填入 BACKEND_BASE_URL（指向 Go 后端，默认 http://localhost:8080）
npm run dev            # 启动 Vite 开发服务器，默认 http://localhost:5173
```
> Vite 已将 `/api` 代理到 Go 后端 `:8080`，本地联调无需跨域配置。生产构建：`npm run build`，产物在 `frontend/dist/`。

### 后端（Go，见 `backend/`）
```bash
cd backend
make db-up     # 本地 Docker 起 MySQL 开发服务器
cp .env.example .env   # 填入第三方 Key
make run       # 启动服务, /health 返回 200
```
详见 [`backend/README.md`](backend/README.md)。

### 测试
- **后端**：`cd backend && go test ./...`（MB1-MB5 各服务均含单元测试）。
- **前端**：测试框架待接入（MVP 早期以手动验证 + 类型检查 `npm run build` 为主）。

### 当前可运行内容
- **前端**：React 三屏（发现 / 图鉴 / 地图）+ 顶部状态栏（体力/金币/城市天气）+ 底部 5 Tab（我的/图鉴/相机凸出/战斗/商店），暖色橙卡通主题；发现屏已接入真实相机（getUserMedia）取帧，体力/经济为前端 mock 逻辑。
- **后端**：Go 后端脚手架（F6）已落地——`/health`、鉴权（MB1）、第三方 API 代理（MB2）、AI 推理编排（MB3）、同步与反作弊审计（MB4）均已实现，详见 `backend/README.md`。

> Foundation 阶段（F1-F6）目标：搭好工程地基。前端配置/存储/日志骨架就位（React Context + IndexedDB）；Go 后端作为联网服务总枢纽，承载全部第三方 Key 与中间件链，为 MVP（M1-M14、MB1-MB5）铺路。
