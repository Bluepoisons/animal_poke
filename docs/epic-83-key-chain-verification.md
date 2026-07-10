# Epic #83 关键链路验收记录

日期：2026-07-10  
分支：`feat/epic-83-close-key-chain`

## 目标链路

```text
React 前端（仅 API 地址 + 设备 Token）
  → Go 后端（运行时读取 Secret）
    → 腾讯地图 / 彩云天气 / Vision 模型 / Text 模型
```

## 子 Issue 状态

P0 #84–#107、P1 #108–#138、P2 #139–#146：**全部 CLOSED**。

## 前端验收对照

| 标准 | 证据 |
|------|------|
| 仅公开配置进入 Vite | `frontend/.env.example` 仅 `VITE_API_BASE_URL` / `VITE_LOG_LEVEL`；`publicConfig.ts` 拦截敏感 `VITE_*` |
| 构建扫描无第三方 Key | `npm run build` → `scripts/scan-bundle-secrets.mjs` |
| 设备注册 + Token | `auth/deviceAuth.ts`：`registerDevice` / `ensureAuth` / singleflight |
| 统一 API Client | `api/client.ts` + `authedRequest`（Bearer、Request-ID、401 续签一次） |
| Detect→Analyze→Value→Sync | `visionDetect.ts` / `capturePipeline.ts` / `syncQueue.ts` |
| 受保护调用均带 Token | geo/weather/vision/value/sync 均走 `authedRequest` 或 `getAccessToken` |
| PWA 不缓存鉴权 API | `vite.config.ts` workbox 不对 `/api/` 做通用缓存 |

## 后端验收对照

| 标准 | 证据 |
|------|------|
| 第三方 Key 仅服务端 | `backend/internal/config/config.go` |
| 生产禁止静默 Mock | `Validate()` + `MockAllowed()` 仅 development |
| Liveness / Readiness 分离 | `handlers/health.go`：`/health`/`/livez` vs readiness |
| OpenAPI 契约 | `docs/openapi.yaml` + `frontend` `openapi:gen` |

## 自动化验证

```bash
cd frontend && npm test   # 43 files / 488+ tests
cd backend && go test ./...
```

## 结论

Epic #83 可执行子项与核心验收标准已在 main 落地；本分支将残余 raw `fetch` 调用统一到 `authedRequest`，并完成本验收记录后关闭 Epic。
