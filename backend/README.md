# animal_poke 后端服务(Go 1.23)

所有联网服务的总枢纽(Gin + Gorm + godotenv + MySQL)。客户端只与本后端通信,
任何第三方 Key(腾讯地图/彩云/Vision/LLM)只存于本工程 `.env`, 客户端永不含第三方 Key。

## 目录结构

```
backend/
├── cmd/main.go
├── docker-compose.yml
├── Makefile
├── .env.example              # 服务端配置唯一本地来源
├── .dockerignore
├── go.mod                    # go 1.23
└── internal/
    ├── config/               # 配置 + Validate + ReadyErrors + CORS 白名单
    ├── middleware/           # RequestID / Logger / Recovery / CORS / JWT / RateLimit / Metrics
    ├── routes/               # 路由装配（依赖缺失返回 503 而非 404）
    ├── handlers/             # health/livez/readyz, auth, geo, weather, vision, value, sync, privacy, commerce, admin
    ├── services/             # Geo / Weather / AI / Audit
    ├── repo/                 # GORM
    └── migrate/              # 版本化迁移
```

## 快速开始

```bash
make db-up
cp .env.example .env   # 填入第三方 Key；开发可 AI_MOCK_ENABLED=true
make run
```

探针：
- `GET /livez`（或 `/health`）— 进程存活
- `GET /readyz`（或 `/ready`）— DB / 配置就绪
- `GET /metrics` — Prometheus 文本指标

## Docker 构建（唯一约定）

```bash
# 在仓库根目录执行，context 必须是 backend/
docker build -f deploy/Dockerfile -t animal-poke-backend:local ./backend
```

## 测试

```bash
make test              # go test ./...
go test -race ./...
go vet ./...
```

## 配置要点

- `SERVER_ADDR=:8080`（不是 `PORT`）
- `CORS_ALLOWED_ORIGINS`：生产必填精确 Origin 列表
- Vision/LLM：`VISION_*` / `LLM_*`
- 详见 `.env.example` 与根 README / OpenAPI

## 中间件链

`RequestID -> Logger -> Recovery -> CORS(allowlist)`

## API 契约

见仓库根 `docs/openapi.yaml`。
