# Requirements Traceability Matrix (AP-060)

> 设计要求 → 代码 → 自动测试 → 运行指标 → Owner。  
> 基线：2026-07-10。完成状态以 **可运行验证命令** 为准，不以历史 Godot 勾选为准。

## Legend

| Status | Meaning |
|--------|---------|
| done | 代码在 main 或已开 PR，有自动化验证 |
| in_progress | 进行中 / 有 PR |
| blocked | 依赖外部（真机/集群/第三方） |
| n/a | 当前里程碑不要求 |

Owner 默认：GitHub assignee；未分配写 `team`。

---

## P0 / Core loop & privacy

| ID | Requirement | Design ref | Code paths | Test command | Metric / dashboard | Owner | Status |
|----|-------------|------------|------------|--------------|--------------------|-------|--------|
| R-01 | 生产入口仅 AnimalPokeApp | 游戏开发计划 4.x / v1.4 | `frontend/src/App.tsx`, `features/animal-poke/AnimalPokeApp.tsx` | `cd frontend && npx vitest run src/features/animal-poke` | — | team | done |
| R-02 | 发现→捕获→收藏主循环 | 3.1 核心循环 | `captureFlow.ts`, `services/capturePipeline.ts`, `services/syncQueue.ts` | `cd frontend && npm run test:e2e` | funnel metrics (AP-035) | team | done |
| R-03 | 客户端零第三方 Key | 2.3 / 隐私 | `frontend/src/config/publicConfig.ts`, backend `.env` only | `cd frontend && npm run scan:secrets` | — | team | done |
| R-04 | 设备鉴权 JWT | 4.3 在线优先 | `backend/internal/handlers/auth.go`, `frontend/src/auth/deviceAuth.ts` | `cd backend && go test ./internal/handlers -run Auth` | auth_ms (k6) | team | done |
| R-05 | 照片经后端 VLM 且即时销毁策略 | 4.5 隐私 | `backend/internal/handlers/vision.go`, AI service | `cd backend && go test ./internal/handlers -run Vision` | AI cost / empty detect | team | done |
| R-06 | 同步幂等 + 服务端权威 | 同步 / 反作弊 | `backend/internal/handlers/sync.go`, `frontend/src/services/syncQueue.ts` | `cd backend && go test ./internal/handlers -run Sync` | sync queue age | team | done |
| R-07 | 体力权威数值 | 3.x 体力 | `frontend/src/stamina/constants.ts`（max 120→240, cost 20, +10/h） | `cd frontend && npx vitest run src/stamina src/capture/session.test.ts` | — | team | done |
| R-08 | 隐私导出/删除 | 合规 | `backend/internal/handlers/privacy.go`, `docs/legal/` | `cd backend && go test ./internal/handlers -run Privacy` | privacy request lag | team | done |
| R-09 | 敏感 API 不被 SW 缓存 | PWA | `frontend/vite.config.ts` workbox | `cd frontend && npx vitest run src/pwa/cachePolicy.test.ts` | — | team | done |
| R-10 | OpenAPI 与 Gin 双向契约 | API 契约 | `docs/openapi.yaml`, `scripts/openapi-contract-gate.mjs` | `node scripts/openapi-contract-gate.mjs` | CI OpenAPI job | team | in_progress |

## P1 platform / quality

| ID | Requirement | Design ref | Code paths | Test command | Metric / dashboard | Owner | Status |
|----|-------------|------------|------------|--------------|--------------------|-------|--------|
| R-11 | Staging→canary→prod + rollback | Release | `.github/workflows/release.yml`, `deploy/scripts/canary-rollback.sh` | `./deploy/scripts/canary-rollback.sh --help` | release SHA gauge | team | in_progress |
| R-12 | 真实指标 / SLO / 告警 | Observability | backend observe helpers, `docs/runbooks/slo-and-alerts.md` | `cd backend && go test ./internal/...` | Prometheus / alerts | team | in_progress |
| R-13 | 客户端错误 schema + 私有 sourcemap | Telemetry | `frontend/src/errors`, backend errors handler | `cd frontend && npx vitest run src/errors` | error rate by release | team | in_progress |
| R-14 | 浏览器矩阵 + 捕获中受控升级 | PWA Mobile QA | Playwright, vite-plugin-pwa | `cd frontend && npm run test:e2e` | — | isaac-sun | in_progress |
| R-15 | 负载与 bundle 预算门禁 | Perf QA | `deploy/loadtest/`, bundle budget script | `k6 run deploy/loadtest/k6-smoke.js` | p95/p99, bundle gzip | isaac-sun | in_progress |
| R-16 | 版本化游戏配置 / Feature Flag | Live Ops | `featureFlags.ts`, backend FeatureFlags | `cd frontend && npx vitest run src/features/animal-poke/featureFlags.test.ts` | config version | isaac-sun | in_progress |

## Docs hygiene

| ID | Requirement | Design ref | Code paths | Test command | Metric / dashboard | Owner | Status |
|----|-------------|------------|------------|--------------|--------------------|-------|--------|
| R-17 | 当前任务不引用 Godot 完成态 | AP-060 | `docs/项目开发任务清单.md`, `docs/archive/godot-foundation-2026-07.md` | `rg -n "Godot 实现已完成" docs/项目开发任务清单.md` 应无匹配 | — | isaac-sun | done |
| R-18 | 文档相对链接可解析 | AP-060 | `docs/**`, `scripts/check-docs-links.mjs` | `node scripts/check-docs-links.mjs` | CI optional | isaac-sun | done |
| R-19 | 体力数值文档与代码一致 | 设计 vs code | stamina constants + task list note | manual RTM row R-07 | — | team | done |

---

## Numeric supersede notes

| Topic | Old design text | Code authority |
|-------|-----------------|----------------|
| Stamina pool | 「10 点体力 + 每日重置」 | `LEVEL_TABLE[0].maxStamina = 120`, hourly +10, capture cost 20 |
| Frontend stack | Godot 4.x scenes | React/PWA `frontend/` |

---

## How to update

1. New P0/P1 requirement → add RTM row **before** coding.  
2. Merge PR → set Status `done` and fill exact test command.  
3. Never mark done solely because an archive Godot block says 完成.
