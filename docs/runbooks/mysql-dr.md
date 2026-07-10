# MySQL 备份 / PITR / 恢复演练

## 目标
- **RPO** ≤ 15 分钟（binlog / 云 PITR）
- **RTO** ≤ 60 分钟（演练环境拉起 + 校验）

## 生产建议
1. 优先托管 MySQL（阿里云 RDS / 云 SQL），开启：
   - 自动全量备份（每日）
   - binlog / PITR
   - 存储加密 + 备份加密
   - 跨可用区保留 ≥ 7 天
2. 应用侧：`AUTO_MIGRATE=false`，迁移 Job 与备份解耦。
3. 备份账号最小权限；凭据只在 Secret Manager。

## 逻辑备份（集群内 CronJob）
见 `deploy/k8s/mysql-backup-cronjob.yaml`。将产物上传到加密对象存储。

## 季度恢复演练清单
1. 在隔离 namespace 创建临时 MySQL。
2. 从最近全量 + binlog 恢复到指定时间点。
3. 校验表：`devices`, `animals`, `audit_logs`, `inferences`, `schema_migrations`。
4. 启动 backend，`/readyz`=200，抽查 sync pull。
5. 记录实际 RPO/RTO 与问题单。

## 告警
- 备份 Job 失败
- 备份年龄 > 26h
- 恢复演练超过 RTO

## 演练记录
| 日期 | RPO | RTO | 结果 | 操作者 |
|------|-----|-----|------|--------|
|  |  |  |  |  |
