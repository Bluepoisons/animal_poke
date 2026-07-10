# 负载测试 Runbook

1. 部署 staging 后端与 MySQL。
2. `k6 run -e BASE_URL=https://api.staging... -e VUS=50 -e DURATION=10m deploy/loadtest/k6-smoke.js`
3. 观察 metrics、HPA、DB 连接、限流 429。
   - **AP-036**: 公网 / Ingress 上的 `GET /metrics` 返回 **404**。
   - 抓取集群内 management 端口：`http://animal-poke-backend-metrics.<ns>.svc:9090/metrics`
     （`METRICS_ADDR`，默认 `:9090`，ClusterIP，勿挂 Ingress）。
4. 将结果填入 `deploy/loadtest/capacity-model.md` 表格。
5. 失败时回滚镜像 tag 并缩小 VUS 复测。
