# 负载测试 Runbook（AP-058）

## 脚本

| 脚本 | 用途 |
|------|------|
| `deploy/loadtest/k6-smoke.js` | 连通性 smoke；geo/weather 允许 503（仅探活） |
| `deploy/loadtest/k6-core-loop.js` | **业务门禁**：auth 必须 200；geo/weather 503 不算成功；sync 4xx 不算成功；意外 5xx 失败 |

## 运行

```bash
# 1) 业务路径（推荐 PR/夜间门禁）
k6 run -e BASE_URL=http://localhost:8080 -e VUS=20 -e DURATION=2m deploy/loadtest/k6-core-loop.js

# 2) 可选 Vision（需 fixture base64）
k6 run -e BASE_URL=... -e ENABLE_VISION=1 -e FIXTURE_JPEG_B64=... deploy/loadtest/k6-core-loop.js

# 3) 旧 smoke
k6 run -e BASE_URL=http://localhost:8080 -e VUS=50 -e DURATION=10m deploy/loadtest/k6-smoke.js
```

## 指标含义

- `business_success`：业务步骤成功比率（auth/geo 200 等）
- `unexpected_5xx`：非预期 5xx
- 默认阈值：`business_success>0.90`，`unexpected_5xx<0.02`，p95<800ms，p99<2s

## Metrics 抓取

- **AP-036**: 公网 / Ingress 上的 `GET /metrics` 返回 **404**。
- 抓取集群内 management 端口：`http://animal-poke-backend-metrics.<ns>.svc:9090/metrics`

## 前端预算

```bash
cd frontend && npm run build && npm run check:bundle-budget
```

- 默认：全部 JS gzip 合计 < 250KB；最大单 chunk gzip < 180KB
- 覆盖：`BUNDLE_JS_GZIP_MAX_KB` / `BUNDLE_ENTRY_GZIP_MAX_KB`

## 记录

将结果填入 `deploy/loadtest/capacity-model.md`。失败时回滚镜像 tag 并缩小 VUS 复测。
