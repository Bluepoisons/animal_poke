# 负载测试 Runbook

1. 部署 staging 后端与 MySQL。
2. `k6 run -e BASE_URL=https://api.staging... -e VUS=50 -e DURATION=10m deploy/loadtest/k6-smoke.js`
3. 观察 `/metrics`、HPA、DB 连接、限流 429。
4. 将结果填入 `deploy/loadtest/capacity-model.md` 表格。
5. 失败时回滚镜像 tag 并缩小 VUS 复测。
