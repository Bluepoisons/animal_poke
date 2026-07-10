# 容量模型（10 万 DAU）

## 假设
- 10 万 DAU，峰值 10% 同时在线 → 1 万 并发会话
- 峰值小时内人均：auth 0.2 + geo 2 + weather 1 + detect 0.5 + sync 0.5 ≈ 4.2 请求
- 峰值小时 RPS ≈ 100000 * 0.1 * 4.2 / 3600 ≈ **12 RPS 平均**；尖峰取 5× → **~60 RPS**

## 当前 HPA
- 后端 3–20 Pod，CPU 70% / Memory 80%
- 单 Pod 目标：~50–80 RPS（轻量 geo/weather mock），含 Vision 时显著下降

## 压测入口
```bash
# 需本地/k8s 后端；DB 可用时 auth 才会 200
k6 run -e BASE_URL=http://localhost:8080 -e VUS=50 -e DURATION=5m deploy/loadtest/k6-smoke.js
```

## 验收记录模板
| 指标 | 目标 | 实测 |
|------|------|------|
| P95 | < 800ms | |
| P99 | < 2s | |
| 错误率 | < 5% | |
| HPA 扩容 | < 60s | |
| DB 连接池 | 无耗尽 | |

## 推荐阈值
- 先按 CPU HPA 保持；若 Vision 上线，为 `/vision/*` 单独限流 + 独立 HPA 指标（RPS/队列）。
- MySQL `MaxOpenConns` 默认 25/Pod，20 Pod → 500；需与 DB `max_connections` 对齐。
