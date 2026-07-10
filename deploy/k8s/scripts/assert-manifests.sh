#!/usr/bin/env bash
# assert-manifests.sh — validate rendered kustomize overlays for AP-011 isolation + AP-015 backup.
#
# Usage:
#   IMAGE_TAG=<sha> ./deploy/k8s/scripts/assert-manifests.sh
#   IMAGE_TAG=<sha> ./deploy/k8s/scripts/assert-manifests.sh /tmp/prod.yaml /tmp/staging.yaml
#
# When no YAML paths are given, builds overlays with kustomize (IMAGE_TAG required).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
K8S_DIR="${ROOT}/deploy/k8s"
IMAGE_TAG="${IMAGE_TAG:-}"
REGISTRY_BACKEND="registry.cn-beijing.aliyuncs.com/animal-poke/backend"
REGISTRY_FRONTEND="registry.cn-beijing.aliyuncs.com/animal-poke/frontend"

# Temp files cleaned on EXIT (set only when we create them).
PROD_YAML=""
STAGING_YAML=""
cleanup() {
  [[ -n "${PROD_YAML}" && -f "${PROD_YAML}" ]] && rm -f "${PROD_YAML}"
  [[ -n "${STAGING_YAML}" && -f "${STAGING_YAML}" ]] && rm -f "${STAGING_YAML}"
}
trap cleanup EXIT

fail() { echo "ASSERT FAIL: $*" >&2; exit 1; }
pass() { echo "ASSERT OK: $*"; }

require_image_tag() {
  if [[ -z "${IMAGE_TAG}" ]]; then
    fail "IMAGE_TAG must be set to an immutable commit SHA or digest (not dev/ci/latest)"
  fi
  case "${IMAGE_TAG}" in
    dev|ci|latest|0.0.0-unset|__IMAGE_TAG__|*placeholder*)
      fail "IMAGE_TAG='${IMAGE_TAG}' is a forbidden mutable/placeholder tag"
      ;;
  esac
}

build_overlay() {
  local overlay="$1"
  local out="$2"
  local work
  work="$(mktemp -d)"
  # Copy full k8s tree so kustomize edit does not dirty the git worktree.
  cp -a "${K8S_DIR}/." "${work}/k8s/"
  (
    cd "${work}/k8s/overlays/${overlay}"
    kustomize edit set image \
      "${REGISTRY_BACKEND}:${IMAGE_TAG}" \
      "${REGISTRY_FRONTEND}:${IMAGE_TAG}"
    kustomize build . > "${out}"
  )
  rm -rf "${work}"
  test -s "${out}" || fail "empty build output for ${overlay}"
}

assert_no_match() {
  local file="$1"
  local pattern="$2"
  local msg="$3"
  if grep -E -q -- "${pattern}" "${file}"; then
    echo "--- offending lines ---" >&2
    grep -E -n -- "${pattern}" "${file}" >&2 || true
    fail "${msg}"
  fi
  pass "${msg}"
}

assert_match() {
  local file="$1"
  local pattern="$2"
  local msg="$3"
  if ! grep -E -q -- "${pattern}" "${file}"; then
    fail "${msg}"
  fi
  pass "${msg}"
}

assert_common() {
  local file="$1"
  local env_name="$2"

  assert_no_match "${file}" ':dev([[:space:]]|$|"|'\'')' \
    "${env_name}: no :dev image tags"
  assert_no_match "${file}" ':latest([[:space:]]|$|"|'\'')' \
    "${env_name}: no :latest image tags"
  assert_no_match "${file}" ':ci([[:space:]]|$|"|'\'')' \
    "${env_name}: no :ci image tags"
  assert_no_match "${file}" '0\.0\.0-unset' \
    "${env_name}: placeholder 0.0.0-unset must be overridden"
  assert_no_match "${file}" '__IMAGE_TAG__' \
    "${env_name}: no __IMAGE_TAG__ placeholders"
  assert_no_match "${file}" '__API_HOST__|__APP_HOST__' \
    "${env_name}: no host placeholders"

  # Release images must use the injected IMAGE_TAG
  assert_match "${file}" "${REGISTRY_BACKEND}:${IMAGE_TAG}" \
    "${env_name}: backend image uses IMAGE_TAG=${IMAGE_TAG}"
  assert_match "${file}" "${REGISTRY_FRONTEND}:${IMAGE_TAG}" \
    "${env_name}: frontend image uses IMAGE_TAG=${IMAGE_TAG}"

  # Every animal-poke image must carry IMAGE_TAG
  local bad_images
  bad_images="$(grep -E 'image:.*animal-poke/(backend|frontend)' "${file}" | grep -v ":${IMAGE_TAG}" || true)"
  if [[ -n "${bad_images}" ]]; then
    echo "${bad_images}" >&2
    fail "${env_name}: found animal-poke images not tagged with ${IMAGE_TAG}"
  fi
  pass "${env_name}: all animal-poke images match IMAGE_TAG"
}

assert_production() {
  local file="$1"
  assert_common "${file}" "production"
  assert_match "${file}" 'DB_HOST:[[:space:]]*"?mysql\.production\.svc\.cluster\.local' \
    "production: DB_HOST is mysql.production.svc.cluster.local"
  assert_match "${file}" 'name:[[:space:]]*backend-secrets' \
    "production: uses backend-secrets"
  assert_no_match "${file}" 'staging-backend-secrets' \
    "production: must not reference staging-backend-secrets"
  assert_no_match "${file}" 'mysql\.staging\.svc' \
    "production: must not reference staging mysql"
  assert_match "${file}" 'kind:[[:space:]]*CronJob' \
    "production: includes mysql-backup CronJob"
  assert_match "${file}" 'name:[[:space:]]*mysql-backup' \
    "production: CronJob named mysql-backup"
  assert_match "${file}" 'namespace:[[:space:]]*production' \
    "production: resources in production namespace"

  # AP-015: durable encrypted object-storage backup (not emptyDir-only)
  assert_match "${file}" 'aws s3 cp' \
    "production: backup uploads via aws s3 cp"
  assert_match "${file}" 'AES256' \
    "production: backup uses SSE (AES256)"
  assert_match "${file}" 'sha256' \
    "production: backup computes checksum"
  assert_match "${file}" 'BACKUP_OK' \
    "production: backup emits BACKUP_OK success marker"
  assert_match "${file}" 'S3_BUCKET' \
    "production: backup targets S3_BUCKET"
  assert_match "${file}" 'mysql-backup-secrets' \
    "production: backup uses mysql-backup-secrets"
  assert_match "${file}" '\*/15 \* \* \* \*' \
    "production: backup schedule supports RPO<=15m"
  assert_no_match "${file}" 'TODO: aws s3|TODO:.*rclone' \
    "production: backup must not leave S3 upload as TODO"
}

assert_staging() {
  local file="$1"
  assert_common "${file}" "staging"

  # Isolation: never production DB / production secret / production host DNS
  assert_no_match "${file}" 'mysql\.production\.svc' \
    "staging: no mysql.production.svc references"
  assert_no_match "${file}" '\.production\.svc' \
    "staging: no *.production.svc references"
  assert_no_match "${file}" 'name:[[:space:]]*backend-secrets([[:space:]]|$)' \
    "staging: must not reference production secret name backend-secrets"
  assert_no_match "${file}" 'namespace:[[:space:]]*production' \
    "staging: no production namespace (should be staging)"

  assert_match "${file}" 'DB_HOST:[[:space:]]*"?mysql\.staging\.svc\.cluster\.local' \
    "staging: DB_HOST is mysql.staging.svc.cluster.local"
  assert_match "${file}" 'DB_NAME:[[:space:]]*"?animal_poke_staging' \
    "staging: DB_NAME is animal_poke_staging"
  assert_match "${file}" 'DB_USER:[[:space:]]*"?animal_poke_staging' \
    "staging: DB_USER is animal_poke_staging"
  assert_match "${file}" 'APP_ENV:[[:space:]]*"?staging' \
    "staging: APP_ENV is staging"
  assert_match "${file}" 'staging-backend-secrets' \
    "staging: uses staging-backend-secrets"
  assert_match "${file}" 'kind:[[:space:]]*NetworkPolicy' \
    "staging: includes NetworkPolicy isolation"
  assert_no_match "${file}" 'kind:[[:space:]]*CronJob' \
    "staging: must not include production mysql-backup CronJob"
  assert_no_match "${file}" 'mysql-backup' \
    "staging: must not reference mysql-backup resources"
}

assert_repo_backup_artifacts() {
  local cron="${K8S_DIR}/overlays/production/mysql-backup-cronjob.yaml"
  local kustom="${K8S_DIR}/overlays/production/kustomization.yaml"
  local staging_kustom="${K8S_DIR}/overlays/staging/kustomization.yaml"
  local drill="${ROOT}/deploy/scripts/backup-restore-drill.sh"
  local runbook="${ROOT}/docs/runbooks/backup-and-dr.md"
  local alerts="${K8S_DIR}/alerts/mysql-backup-alerts.example.yaml"

  [[ -f "${cron}" ]] || fail "missing ${cron}"
  [[ -f "${kustom}" ]] || fail "missing ${kustom}"
  [[ -f "${drill}" ]] || fail "missing ${drill}"
  [[ -x "${drill}" ]] || fail "backup-restore-drill.sh must be executable"
  [[ -f "${runbook}" ]] || fail "missing ${runbook}"
  [[ -f "${alerts}" ]] || fail "missing ${alerts}"

  grep -q 'mysql-backup-cronjob.yaml' "${kustom}" \
    || fail "production kustomization must include mysql-backup-cronjob.yaml"
  pass "production kustomization includes mysql-backup-cronjob.yaml"

  if grep -q 'mysql-backup' "${staging_kustom}" 2>/dev/null; then
    fail "staging kustomization must not include mysql-backup"
  fi
  pass "staging kustomization excludes mysql-backup"

  grep -Eq 'RPO.*15|≤ 15|<= 15' "${runbook}" || fail "runbook must document RPO <= 15m"
  grep -Eq 'RTO.*60|≤ 60|<= 60' "${runbook}" || fail "runbook must document RTO <= 60m"
  pass "runbook documents RPO<=15m and RTO<=60m"

  grep -q 'aws s3 cp' "${cron}" || fail "cronjob must upload with aws s3 cp"
  grep -q 'AES256' "${cron}" || fail "cronjob must use SSE AES256"
  grep -q 'sha256' "${cron}" || fail "cronjob must checksum"
  grep -q 'S3_BUCKET' "${cron}" || fail "cronjob must target S3_BUCKET"
  grep -q 'BACKUP_OK' "${cron}" || fail "cronjob must emit BACKUP_OK"
  pass "cronjob has S3 SSE checksum path (not emptyDir-only)"
}

main() {
  require_image_tag
  assert_repo_backup_artifacts

  if [[ $# -ge 2 ]]; then
    PROD_YAML=""
    STAGING_YAML=""
    local prod_file="$1"
    local staging_file="$2"
    echo "=== Asserting production manifest: ${prod_file} ==="
    assert_production "${prod_file}"
    echo "=== Asserting staging manifest: ${staging_file} ==="
    assert_staging "${staging_file}"
  else
    PROD_YAML="$(mktemp)"
    STAGING_YAML="$(mktemp)"
    echo "Building production overlay with IMAGE_TAG=${IMAGE_TAG}..."
    build_overlay production "${PROD_YAML}"
    echo "Building staging overlay with IMAGE_TAG=${IMAGE_TAG}..."
    build_overlay staging "${STAGING_YAML}"
    echo "=== Asserting production manifest: ${PROD_YAML} ==="
    assert_production "${PROD_YAML}"
    echo "=== Asserting staging manifest: ${STAGING_YAML} ==="
    assert_staging "${STAGING_YAML}"
  fi
  echo "All k8s isolation + backup assertions passed."
}

main "$@"
