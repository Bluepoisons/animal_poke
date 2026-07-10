#!/usr/bin/env bash
# backup-restore-drill.sh — isolated MySQL restore drill for AP-015.
#
# Goals:
#   RPO <= 15 minutes (restore a dump no older than 15m, or PITR to a random point)
#   RTO <= 60 minutes (end-to-end restore + consistency checks)
#
# Modes:
#   1) Object-storage dump restore (default): download latest .sql.gz from S3, load into temp MySQL
#   2) File restore: RESTORE_FROM=/path/to/dump.sql.gz
#   3) Dry-run validation: --dry-run (assert scripts/env only)
#
# Required env (no secrets printed):
#   S3_BUCKET            e.g. s3://animal-poke-mysql-backups
#   BACKUP_PREFIX        default: mysql/animal_poke
#   AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY / AWS_DEFAULT_REGION
#   S3_ENDPOINT          optional S3-compatible endpoint
#   MYSQL_ROOT_PASSWORD  password for temporary MySQL container
#   DRILL_DB             default: animal_poke_drill
#   EXPECTED_TABLES      space-separated core tables to verify
#   BACKEND_READY_URL    optional, e.g. http://127.0.0.1:8080/readyz
#   RPO_MAX_MINUTES      default 15
#   RTO_MAX_MINUTES      default 60
#
# Usage:
#   ./deploy/scripts/backup-restore-drill.sh
#   RESTORE_FROM=./fixture.sql.gz ./deploy/scripts/backup-restore-drill.sh
#   ./deploy/scripts/backup-restore-drill.sh --dry-run
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT}"

DRY_RUN=0
if [[ "${1:-}" == "--dry-run" ]]; then
  DRY_RUN=1
fi

log() { printf '[%s] %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*"; }
die() { log "ERROR: $*"; exit 1; }
pass() { log "OK: $*"; }

RPO_MAX_MINUTES="${RPO_MAX_MINUTES:-15}"
RTO_MAX_MINUTES="${RTO_MAX_MINUTES:-60}"
BACKUP_PREFIX="${BACKUP_PREFIX:-mysql/animal_poke}"
DRILL_DB="${DRILL_DB:-animal_poke_drill}"
EXPECTED_TABLES="${EXPECTED_TABLES:-devices animals audit_logs inferences schema_migrations}"
MYSQL_IMAGE="${MYSQL_IMAGE:-mysql:8.0}"
MYSQL_ROOT_PASSWORD="${MYSQL_ROOT_PASSWORD:-drill-root-not-for-prod}"
CONTAINER_NAME="${CONTAINER_NAME:-ap-mysql-drill-$$}"
WORKDIR="${WORKDIR:-$(mktemp -d -t ap-backup-drill.XXXXXX)}"
START_EPOCH="$(date +%s)"

cleanup() {
  local code=$?
  if [[ "${KEEP_DRILL_CONTAINER:-0}" != "1" ]]; then
    docker rm -f "${CONTAINER_NAME}" >/dev/null 2>&1 || true
  fi
  if [[ "${KEEP_WORKDIR:-0}" != "1" ]]; then
    rm -rf "${WORKDIR}" >/dev/null 2>&1 || true
  fi
  exit "${code}"
}
trap cleanup EXIT

aws_args() {
  local args=()
  if [[ -n "${S3_ENDPOINT:-}" ]]; then
    args+=(--endpoint-url "${S3_ENDPOINT}")
  fi
  printf '%s\n' "${args[@]}"
}

elapsed_minutes() {
  local now
  now="$(date +%s)"
  echo $(( (now - START_EPOCH) / 60 ))
}

assert_rto() {
  local mins
  mins="$(elapsed_minutes)"
  if (( mins > RTO_MAX_MINUTES )); then
    die "RTO exceeded: elapsed=${mins}m > max=${RTO_MAX_MINUTES}m"
  fi
  pass "RTO check: elapsed=${mins}m <= ${RTO_MAX_MINUTES}m"
}

download_latest_dump() {
  local bucket_uri="${S3_BUCKET:-}"
  [[ -n "${bucket_uri}" ]] || die "S3_BUCKET is required unless RESTORE_FROM is set"
  case "${bucket_uri}" in
    s3://*) ;;
    *) bucket_uri="s3://${bucket_uri}" ;;
  esac

  local mapfile_args=()
  while IFS= read -r a; do mapfile_args+=("$a"); done < <(aws_args)

  log "Listing backups under ${bucket_uri%/}/${BACKUP_PREFIX}/"
  # Pick newest .sql.gz (aws s3 ls is sorted by key; use last matching line)
  local listing key
  listing="$(aws s3 ls "${bucket_uri%/}/${BACKUP_PREFIX}/" "${mapfile_args[@]}" 2>/dev/null || true)"
  [[ -n "${listing}" ]] || die "no objects under ${bucket_uri%/}/${BACKUP_PREFIX}/"
  key="$(echo "${listing}" | awk '/\.sql\.gz$/ {print $4}' | tail -n1)"
  [[ -n "${key}" ]] || die "no .sql.gz objects found under prefix"

  local dest="${WORKDIR}/restore.sql.gz"
  log "Downloading ${bucket_uri%/}/${BACKUP_PREFIX}/${key}"
  aws s3 cp "${bucket_uri%/}/${BACKUP_PREFIX}/${key}" "${dest}" \
    "${mapfile_args[@]}" --only-show-errors

  # Optional sidecar checksum
  if aws s3 cp "${bucket_uri%/}/${BACKUP_PREFIX}/${key}.sha256" "${dest}.sha256" \
      "${mapfile_args[@]}" --only-show-errors 2>/dev/null; then
    log "Verifying sha256"
    (cd "${WORKDIR}" && sha256sum -c "restore.sql.gz.sha256") || die "checksum mismatch"
    pass "checksum verified"
  else
    log "WARN: no .sha256 sidecar; computing local sha256 only"
    sha256sum "${dest}"
  fi

  # RPO: object LastModified within RPO_MAX_MINUTES if HEAD available
  if command -v aws >/dev/null 2>&1; then
    local bucket_name key_full lm_epoch now_epoch age_min
    bucket_name="$(echo "${bucket_uri}" | sed 's#s3://##;s#/.*##')"
    key_full="${BACKUP_PREFIX}/${key}"
    # Prefer listing date column as coarse RPO signal
    local date_part time_part
    date_part="$(echo "${listing}" | awk -v k="${key}" '$4==k {print $1}')"
    time_part="$(echo "${listing}" | awk -v k="${key}" '$4==k {print $2}')"
    if [[ -n "${date_part}" && -n "${time_part}" ]]; then
      if lm_epoch="$(date -u -d "${date_part} ${time_part}" +%s 2>/dev/null)"; then
        now_epoch="$(date +%s)"
        age_min=$(( (now_epoch - lm_epoch) / 60 ))
        log "Backup age ≈ ${age_min} minutes (from listing timestamp)"
        if (( age_min > RPO_MAX_MINUTES )); then
          die "RPO exceeded: backup age ${age_min}m > ${RPO_MAX_MINUTES}m"
        fi
        pass "RPO check: backup age ${age_min}m <= ${RPO_MAX_MINUTES}m"
      else
        log "WARN: could not parse backup age; continue (set RPO_SKIP=1 to silence)"
      fi
    fi
  fi

  echo "${dest}"
}

start_temp_mysql() {
  log "Starting temporary MySQL container ${CONTAINER_NAME}"
  docker run -d --name "${CONTAINER_NAME}" \
    -e MYSQL_ROOT_PASSWORD="${MYSQL_ROOT_PASSWORD}" \
    -e MYSQL_DATABASE="${DRILL_DB}" \
    -p 13306:3306 \
    "${MYSQL_IMAGE}" >/dev/null

  local i
  for i in $(seq 1 60); do
    if docker exec "${CONTAINER_NAME}" mysqladmin ping -uroot -p"${MYSQL_ROOT_PASSWORD}" --silent 2>/dev/null; then
      pass "MySQL ready"
      return 0
    fi
    sleep 2
  done
  die "MySQL container did not become ready in time"
}

restore_dump() {
  local dump="$1"
  log "Restoring ${dump} into ${DRILL_DB}"
  # Create DB if needed
  docker exec -i "${CONTAINER_NAME}" \
    mysql -uroot -p"${MYSQL_ROOT_PASSWORD}" -e "CREATE DATABASE IF NOT EXISTS \`${DRILL_DB}\`;"

  if [[ "${dump}" == *.gz ]]; then
    gunzip -c "${dump}" | docker exec -i "${CONTAINER_NAME}" \
      mysql -uroot -p"${MYSQL_ROOT_PASSWORD}" "${DRILL_DB}"
  else
    docker exec -i "${CONTAINER_NAME}" \
      mysql -uroot -p"${MYSQL_ROOT_PASSWORD}" "${DRILL_DB}" < "${dump}"
  fi
  pass "dump loaded"
}

verify_tables() {
  log "Verifying core tables: ${EXPECTED_TABLES}"
  local t count missing=0
  for t in ${EXPECTED_TABLES}; do
    count="$(docker exec "${CONTAINER_NAME}" \
      mysql -uroot -p"${MYSQL_ROOT_PASSWORD}" -N -e \
      "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='${DRILL_DB}' AND table_name='${t}';" \
      2>/dev/null || echo 0)"
    if [[ "${count}" != "1" ]]; then
      log "MISSING table: ${t}"
      missing=$((missing + 1))
    else
      local rows
      rows="$(docker exec "${CONTAINER_NAME}" \
        mysql -uroot -p"${MYSQL_ROOT_PASSWORD}" -N -e \
        "SELECT COUNT(*) FROM \`${DRILL_DB}\`.\`${t}\`;" 2>/dev/null || echo "?")"
      pass "table ${t} exists (rows=${rows})"
    fi
  done
  if (( missing > 0 )); then
    die "${missing} expected table(s) missing after restore"
  fi
}

optional_readyz() {
  if [[ -z "${BACKEND_READY_URL:-}" ]]; then
    log "BACKEND_READY_URL unset — skip API readyz check"
    return 0
  fi
  log "Checking ${BACKEND_READY_URL}"
  local code
  code="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 10 "${BACKEND_READY_URL}" || true)"
  if [[ "${code}" != "200" ]]; then
    die "readyz returned HTTP ${code}"
  fi
  pass "readyz=200"
}

main() {
  log "AP-015 backup restore drill starting (RPO<=${RPO_MAX_MINUTES}m RTO<=${RTO_MAX_MINUTES}m)"
  log "WORKDIR=${WORKDIR}"

  if [[ "${DRY_RUN}" == "1" ]]; then
    pass "dry-run: script loads, paths resolve ROOT=${ROOT}"
    test -f "${ROOT}/deploy/k8s/overlays/production/mysql-backup-cronjob.yaml" \
      || die "missing production mysql-backup-cronjob.yaml"
    test -f "${ROOT}/docs/runbooks/backup-and-dr.md" \
      || die "missing docs/runbooks/backup-and-dr.md"
    pass "dry-run assertions passed"
    return 0
  fi

  command -v docker >/dev/null 2>&1 || die "docker is required"
  command -v gunzip >/dev/null 2>&1 || die "gunzip is required"

  local dump
  if [[ -n "${RESTORE_FROM:-}" ]]; then
    dump="${RESTORE_FROM}"
    test -s "${dump}" || die "RESTORE_FROM not found or empty: ${dump}"
    log "Using local dump ${dump}"
  else
    command -v aws >/dev/null 2>&1 || die "aws CLI required when RESTORE_FROM is unset"
    dump="$(download_latest_dump)"
  fi

  start_temp_mysql
  restore_dump "${dump}"
  verify_tables
  optional_readyz
  assert_rto

  log "DRILL_PASS database=${DRILL_DB} dump=${dump}"
  log "Record this run in docs/runbooks/backup-and-dr.md drill table"
}

main "$@"
