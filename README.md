# animal_poke

LBS 动物收集手游（基于 CatchCat 概念改进）。真实世界探索 + 云端 VLM 实时动物识别 + 云端 LLM 数值生成

> 设计文档单一事实来源：[`docs/游戏开发计划.md`](docs/游戏开发计划.md)  
> 执行层任务清单：[`docs/项目开发任务清单.md`](docs/项目开发任务清单.md)  
> API 契约：[`docs/openapi.yaml`](docs/openapi.yaml)

---

## 技术栈

| 层 | 选型 |
|----|------|
| 前端框架 | React 18 + Vite 6 + TypeScript 5.6（Web/PWA） |
| 物理/捕获 | Canvas 2D（MVP） |
| UI | React 组件 + CSS 变量（暖橙卡通） |
| 相机/定位 | getUserMedia + Geolocation |
| 云端 AI | 云端 VLM + LLM（客户端零本地推理、零第三方 Key） |
| 后端 | Go 1.23（Gin + Gorm） |
| 架构 | 在线优先；断网仅图鉴浏览 |
| 本地存储 | IndexedDB + localStorage |
| 密钥 | **仅** `backend/.env` / K8s Secret；前端仅 `VITE_API_BASE_URL` |

---

## 目录结构

```
animal_poke/
├── frontend/                 # React + Vite
│   ├── .env.example          # 仅公开配置 VITE_API_BASE_URL
│   ├── package.json
│   ├── vite.config.ts        # /api → localhost:8080
│   └── src/
│       ├── main.tsx / App.tsx
│       ├── api/              # OpenAPI 生成类型 + client
│       ├── features/animal-poke/  # 当前生产入口 UI
│       ├── services/         # fetch / vision / antiCheat
│       └── ...               # lbs / weather / shop / battle 等模块
├── backend/                  # Go 后端
│   ├── cmd/
│   ├── internal/{config,routes,handlers,services,middleware,repo,migrate}
│   ├── .env.example
│   └── go.mod                # go 1.23
├── deploy/
│   ├── Dockerfile            # 后端镜像（context=./backend）
│   ├── Dockerfile.frontend   # 前端 Nginx 镜像
│   └── k8s/                  # Namespace / ConfigMap / Deploy / Ingress / overlays
├── docs/
│   ├── openapi.yaml
│   ├── 游戏开发计划.md
│   ├── 项目开发任务清单.md
│   └── runbooks/
├── .github/workflows/ci.yml
└── README.md
```

---

## 客户端零第三方 Key

```
React（VITE_API_BASE_URL + 设备 Token）
  → Go 后端（运行时读 Secret）
    → 腾讯地图 / 彩云 / Vision / LLM
```

- 本地服务端配置**唯一**所有者：`backend/.env`（见 `backend/.env.example`）。
- 前端：`frontend/.env.example` 只有 `VITE_API_BASE_URL`。
- 生产：K8s Secret / 云 Secret Manager；轮换见 [`docs/runbooks/secret-rotation.md`](docs/runbooks/secret-rotation.md)。

---

## 运行项目

### 环境要求
- **Node.js 22 LTS**（推荐），npm 10+
- **Go 1.23.x**
- Docker（可选：MySQL / 镜像构建）

### 后端
```bash
cd backend
cp .env.example .env          # 填入第三方 Key；开发可开 AI_MOCK_ENABLED=true
make db-up                    # 本地 MySQL
make run                      # SERVER_ADDR=:8080
# 探针: GET /livez  /readyz  /metrics
```

### 前端
```bash
cd frontend
cp .env.example .env          # 本地可留空 VITE_API_BASE_URL，走代理
npm install
npm run dev                   # http://localhost:5173
npm test
npm run build
```

### 一条命令构建后端镜像
```bash
docker build -f deploy/Dockerfile -t animal-poke-backend:local ./backend
docker run --rm -p 8080:8080 \
  -e APP_ENV=development -e AI_MOCK_ENABLED=true \
  -e JWT_SECRET=local-dev-secret-at-least-32-chars \
  -e SERVER_ADDR=:8080 \
  animal-poke-backend:local
curl -fsS http://127.0.0.1:8080/livez
```

### 前端生产镜像
见 [`docs/runbooks/frontend-release.md`](docs/runbooks/frontend-release.md)。

### OpenAPI / TS 类型
```bash
npx openapi-typescript docs/openapi.yaml -o frontend/src/api/generated/schema.d.ts
```
本地文档：可用 Redoc/Swagger UI 打开 `docs/openapi.yaml`。

### 测试
- 后端：`cd backend && go test ./...`（另有 `-race` / `vet`）
- 前端：`cd frontend && npm test`（Vitest）；入口覆盖见 `src/features/animal-poke/*.test.tsx`
- CI：`.github/workflows/ci.yml`（backend / frontend / openapi / container / k8s / gitleaks）

---

## 部署

- 后端清单：`deploy/k8s/base/backend.yaml`（`SERVER_ADDR`、Secret 契约、`/livez`+`/readyz`、非 root、只读根 FS）
- 配置：`deploy/k8s/backend-configmap.yaml` + `backend-secret.example.yaml`
- 前端：`deploy/k8s/base/frontend.yaml` + Nginx SPA 回退
- Ingress / 环境：`deploy/k8s/base/ingress.yaml` + `overlays/{staging,production}`
- 镜像 tag：**commit SHA**，禁止 `latest`

---

## 团队规范

- 设计只写 `docs/游戏开发计划.md`；任务更新 `docs/项目开发任务清单.md`
- Git：`git pull --rebase`；见 `CONTRIBUTING.md` / `SECURITY.md`
- UI 主题：主色 `#FF8C42` / 背景 `#FFF8F0` / 文字 `#4A2C1A`；稀有度边框色不改

### 当前能力矩阵（简）
| 能力 | 状态 |
|------|------|
| 后端鉴权 / Geo / Weather / Vision / Value / Sync | 已实现（联调依赖真实 Key 或 Mock） |
| 前端手账 UI 入口 | 可运行；真实 Token/相机/Detect 仍为前端子任务 |
| CI 门禁 | 已配置 |
| 生产 CORS 白名单 | `CORS_ALLOWED_ORIGINS` |

---

## 许可证与安全
- 贡献：[`CONTRIBUTING.md`](CONTRIBUTING.md)
- 安全披露：[`SECURITY.md`](SECURITY.md)
