#!/usr/bin/env bash
# assert-manifests.sh — validate rendered kustomize overlays for AP-011 isolation.
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
  assert_match "${file}" 'namespace:[[:space:]]*production' \
    "production: resources in production namespace"
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
}

main() {
  require_image_tag

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
  echo "All k8s isolation assertions passed."
}

main "$@"
