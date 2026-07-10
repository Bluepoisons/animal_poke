# Backup & Disaster Recovery Runbook (AP-015)

## Objectives

| Metric | Target | Notes |
|--------|--------|-------|
| **RPO** | **≤ 15 minutes** | Logical CronJob every 15m + preferred cloud binlog/PITR |
| **RTO** | **≤ 60 minutes** | Isolated restore + table/API consistency checks |

Backups must be **persistent**, **encrypted at rest**, **non-anonymously accessible**, and **regularly drillable**. Pod `emptyDir` alone is **not** a backup destination.

## Architecture

```
MySQL (production)
    │  mysqldump --single-transaction (every 15 min)
    ▼
CronJob mysql-backup (production overlay only)
    │  gzip + sha256 + meta.json
    │  aws s3 cp --sse AES256
    ▼
S3-compatible object storage (versioned, SSE, least-privilege IAM)
    │
    ▼
Quarterly / on-demand restore drill (deploy/scripts/backup-restore-drill.sh)
    → temp MySQL → table counts → optional /readyz
```

**Primary recommendation:** managed MySQL (Aliyun RDS / Cloud SQL) with:

- Daily automated full backup
- Continuous binlog / PITR
- Storage + backup encryption
- Cross-AZ retention ≥ 7 days

The in-cluster CronJob is a **safety net** and works for self-hosted MySQL.

## Manifests

| Path | Purpose |
|------|---------|
| `deploy/k8s/overlays/production/mysql-backup-cronjob.yaml` | CronJob + SA + secret schema |
| `deploy/k8s/overlays/production/kustomization.yaml` | Includes CronJob (**production only**) |
| `deploy/k8s/overlays/staging/*` | **Must not** include backup CronJob |
| `deploy/k8s/alerts/mysql-backup-alerts.example.yaml` | PrometheusRule / alert examples |
| `deploy/scripts/backup-restore-drill.sh` | Automated restore drill |

Staging isolation is enforced by `deploy/k8s/scripts/assert-manifests.sh`.

## Secrets (never commit real values)

Create `mysql-backup-secrets` in namespace `production` (External Secrets preferred):

```bash
kubectl -n production create secret generic mysql-backup-secrets \
  --from-literal=MYSQL_HOST=mysql.production.svc.cluster.local \
  --from-literal=MYSQL_USER=animal_poke_backup \
  --from-literal=MYSQL_PASSWORD='***' \
  --from-literal=MYSQL_DATABASE=animal_poke \
  --from-literal=AWS_ACCESS_KEY_ID='***' \
  --from-literal=AWS_SECRET_ACCESS_KEY='***' \
  --from-literal=AWS_DEFAULT_REGION=cn-beijing \
  --from-literal=S3_ENDPOINT=https://oss-cn-beijing.aliyuncs.com \
  --from-literal=S3_BUCKET=s3://animal-poke-mysql-backups \
  --from-literal=BACKUP_RETENTION_DAYS=14
```

IAM policy principles:

- Least privilege: `s3:PutObject`, `s3:GetObject`, `s3:ListBucket`, `s3:DeleteObject` (retention only) on the backup prefix
- Deny public ACL / anonymous `GetObject`
- Bucket versioning **on**
- Default encryption **SSE-S3 or SSE-KMS**
- Lifecycle rule: expire noncurrent / objects after retention days (source of truth for retention)

MySQL backup user privileges (minimum):

```sql
-- SELECT, SHOW VIEW, TRIGGER, EVENT, LOCK TABLES (as required by mysqldump)
GRANT SELECT, SHOW VIEW, TRIGGER, EVENT, LOCK TABLES ON animal_poke.* TO 'animal_poke_backup'@'%';
FLUSH PRIVILEGES;
```

## Day-2 operations

### Verify last successful backup

```bash
# Job history
kubectl -n production get cronjob mysql-backup
kubectl -n production get jobs -l app.kubernetes.io/name=mysql-backup --sort-by=.metadata.creationTimestamp | tail

# Logs (look for BACKUP_OK)
kubectl -n production logs job/$(kubectl -n production get jobs -l app.kubernetes.io/name=mysql-backup \
  --sort-by=.metadata.creationTimestamp -o jsonpath='{.items[-1:].metadata.name}')

# Object storage listing
aws s3 ls s3://animal-poke-mysql-backups/mysql/animal_poke/ --endpoint-url "$S3_ENDPOINT" | tail
```

### Manual job trigger

```bash
kubectl -n production create job --from=cronjob/mysql-backup mysql-backup-manual-$(date +%s)
```

### Restore drill (RPO / RTO)

```bash
# Full drill from object storage
export S3_BUCKET=s3://animal-poke-mysql-backups
export S3_ENDPOINT=https://oss-cn-beijing.aliyuncs.com
export AWS_ACCESS_KEY_ID=...
export AWS_SECRET_ACCESS_KEY=...
export AWS_DEFAULT_REGION=cn-beijing
export RPO_MAX_MINUTES=15
export RTO_MAX_MINUTES=60
./deploy/scripts/backup-restore-drill.sh

# Local fixture dump
RESTORE_FROM=/path/to/animal_poke_YYYYMMDD.sql.gz ./deploy/scripts/backup-restore-drill.sh

# CI / static dry-run
./deploy/scripts/backup-restore-drill.sh --dry-run
```

### Core consistency checks after restore

Expected tables (default):

- `devices`
- `animals`
- `audit_logs`
- `inferences`
- `schema_migrations`

Optional: start backend against restored DB and assert `GET /readyz` → 200; sample sync pull.

## Alerts

See `deploy/k8s/alerts/mysql-backup-alerts.example.yaml`.

| Alert | Condition | Severity |
|-------|-----------|----------|
| MySQLBackupJobFailed | CronJob/Job failed | critical |
| MySQLBackupTooOld | No success in > 26h (or > 30m for 15m schedule) | critical |
| MySQLBackupRestoreRTOExceeded | Drill RTO > 60m | warning |
| MySQLBackupObjectPublic | Public ACL / policy detected (cloud config audit) | critical |

Wire log metric `BACKUP_OK` / `mysql_backup_last_success_unixtime` into Prometheus (sidecar textfile or log-based metric).

## Failure scenarios (test matrix)

| Scenario | Expected |
|----------|----------|
| Drop test DB → restore from latest dump | Tables + counts OK, RTO ≤ 60m |
| Corrupt local gzip | Checksum fail; Job fails; alert |
| Invalid IAM / expired key | Upload fails; Job fails; alert |
| Mid-upload network cut | Job fails / incomplete object not marked BACKUP_OK |
| Retention / lifecycle | Objects older than retention eventually removed; recent kept |
| Quarterly drill | Recorded in table below with measured RPO/RTO |

## Application coupling

- Production: `AUTO_MIGRATE=false`; schema migrations run as explicit Jobs, decoupled from backup.
- Never restore production dumps into staging without redaction / isolation.
- Never put production backup CronJob in staging kustomization.

## Drill log

| Date (UTC) | RPO measured | RTO measured | Result | Operator | Notes |
|------------|--------------|--------------|--------|----------|-------|
|  |  |  |  |  |  |

## Rollback

1. Pause CronJob: `kubectl -n production patch cronjob mysql-backup -p '{"spec":{"suspend":true}}'`
2. Revert to previous image/secret if upload path wrong
3. Rely on managed RDS automatic backups if app CronJob is suspended
4. Re-enable after fix; run one manual Job + drill

## Related

- `docs/runbooks/mysql-dr.md` — short pointer / historical notes
- Issue #176 AP-015
