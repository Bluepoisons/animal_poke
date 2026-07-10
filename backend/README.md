# animal_poke 后端服务(Go 1.25)

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
├── go.mod                    # go 1.25
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
- `GET /metrics` on **METRICS_ADDR** (default `:9090`) — Prometheus 文本指标（management only）
- 公网/主端口 `GET /metrics` → **404**（AP-036，不经 Ingress 暴露）

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
make vision-golden-stub  # AP-047 ML golden set (stub, no API keys)
```

### Vision 黄金集（AP-047）

- 清单与基线：`testdata/vision_golden/manifest.json`、`baseline.json`
- PR 门禁：`make vision-golden-stub` / CI job `vision-golden-stub`（mock 提供商）
- 指标：per-class precision/recall、unknown rejection、mean IoU、latency P50/P95/P99、cost 占位
- 退化阈值：基线 diff 超过 `manifest.thresholds` 即失败
- 真实 Provider 认证：见 `.github/workflows/vision-golden-real.yml`（`workflow_dispatch` / nightly 手动），需 `VISION_*` secrets；默认不在 PR 跑

## 配置要点

- `SERVER_ADDR=:8080`（不是 `PORT`）
- `METRICS_ADDR=:9090` — management metrics listen address；设为 `off` 可关闭
- `CORS_ALLOWED_ORIGINS`：生产必填精确 Origin 列表
- Vision/LLM：`VISION_*` / `LLM_*`
- 详见 `.env.example` 与根 README / OpenAPI

## Metrics 安全（AP-036）

- 主路由（Ingress）不再提供可抓取的 `/metrics`（返回 404）
- 独立 `NewMetricsServer` 绑定 `METRICS_ADDR`；K8s 使用 ClusterIP Service `animal-poke-backend-metrics`
- `ObserveHTTP` 仅使用 `c.FullPath()` 路由模板；未匹配 → `unknown`；series 有上限

## 中间件链

`RequestID -> Logger -> Recovery -> CORS(allowlist)`

## API 契约

见仓库根 `docs/openapi.yaml`。
