# MySQL 备份 / PITR / 恢复演练（短版）
#
# 完整 Runbook（RPO ≤ 15m / RTO ≤ 60m、对象存储、演练脚本、告警）见：
#   docs/runbooks/backup-and-dr.md
#
# CronJob（仅 production overlay）：
#   deploy/k8s/overlays/production/mysql-backup-cronjob.yaml
#
# 恢复演练：
#   ./deploy/scripts/backup-restore-drill.sh
#   ./deploy/scripts/backup-restore-drill.sh --dry-run

## 目标
- **RPO** ≤ 15 分钟（15 分钟逻辑备份 + 云 binlog / PITR）
- **RTO** ≤ 60 分钟（隔离环境拉起 + 校验）

## 生产建议
1. 优先托管 MySQL（阿里云 RDS / 云 SQL），开启：
   - 自动全量备份（每日）
   - binlog / PITR
   - 存储加密 + 备份加密
   - 跨可用区保留 ≥ 7 天
2. 应用侧：`AUTO_MIGRATE=false`，迁移 Job 与备份解耦。
3. 备份账号最小权限；凭据只在 Secret Manager。
4. **禁止**仅把备份写在 Pod `emptyDir`：必须上传到版本化、SSE 加密的对象存储。

## 逻辑备份（集群内 CronJob）
见 `deploy/k8s/overlays/production/mysql-backup-cronjob.yaml`（仅 production overlay，禁止引入 staging）。
产物：`.sql.gz` + `.sha256` + `.meta.json`，`aws s3 cp --sse AES256`。

## 季度恢复演练清单
1. 在隔离环境启动临时 MySQL（脚本自动完成）。
2. 从对象存储最近全量（或本地 dump）恢复。
3. 校验表：`devices`, `animals`, `audit_logs`, `inferences`, `schema_migrations`。
4. 可选：启动 backend，`/readyz`=200，抽查 sync pull。
5. 记录实际 RPO/RTO 与问题单到 `docs/runbooks/backup-and-dr.md`。

```bash
./deploy/scripts/backup-restore-drill.sh
```

## 告警
见 `deploy/k8s/alerts/mysql-backup-alerts.example.yaml`：
- 备份 Job 失败
- 备份年龄 > 30 分钟（相对 15 分钟调度）
- 恢复演练超过 RTO
- CronJob 被 suspend

## 演练记录
完整表格见 `docs/runbooks/backup-and-dr.md`。
