#!/usr/bin/env bash
# canary-rollback.sh — one-click / auto-rollback for canary or production (AP-039)
#
# Rolls backend + frontend Deployments back to a known-good immutable image tag
# (commit SHA). Prefer this over `kubectl rollout undo` when the desired previous
# revision is a specific release SHA rather than "N revisions ago".
#
# Usage:
#   ./deploy/scripts/canary-rollback.sh --namespace production --previous-tag <sha>
#   ./deploy/scripts/canary-rollback.sh --dry-run --namespace production --previous-tag <sha>
#   ./deploy/scripts/canary-rollback.sh --help
#
# Environment (optional overrides):
#   BACKEND_IMAGE   default registry.cn-beijing.aliyuncs.com/animal-poke/backend
#   FRONTEND_IMAGE  default registry.cn-beijing.aliyuncs.com/animal-poke/frontend
#   BACKEND_DEPLOY  default animal-poke-backend
#   FRONTEND_DEPLOY default animal-poke-frontend
#   READY_TIMEOUT_SEC default 300
#   KUBECONFIG      standard kubectl config
#
# Exit codes:
#   0 success (or successful dry-run)
#   1 usage / validation error
#   2 kubectl / rollout failure
#   3 post-rollback readiness failure
set -euo pipefail

BACKEND_IMAGE="${BACKEND_IMAGE:-registry.cn-beijing.aliyuncs.com/animal-poke/backend}"
FRONTEND_IMAGE="${FRONTEND_IMAGE:-registry.cn-beijing.aliyuncs.com/animal-poke/frontend}"
BACKEND_DEPLOY="${BACKEND_DEPLOY:-animal-poke-backend}"
FRONTEND_DEPLOY="${FRONTEND_DEPLOY:-animal-poke-frontend}"
READY_TIMEOUT_SEC="${READY_TIMEOUT_SEC:-300}"

NAMESPACE=""
PREVIOUS_TAG=""
REASON="unspecified"
DRY_RUN=0
SKIP_READY=0
BACKEND_ONLY=0
FRONTEND_ONLY=0

log()  { printf '=> %s\n' "$*"; }
warn() { printf 'WARN: %s\n' "$*" >&2; }
die()  { printf 'ERROR: %s\n' "$*" >&2; exit 1; }
fail2(){ printf 'ERROR: %s\n' "$*" >&2; exit 2; }
fail3(){ printf 'ERROR: %s\n' "$*" >&2; exit 3; }

usage() {
  cat <<'EOF'
canary-rollback.sh — roll Deployments back to a previous immutable image tag (AP-039)

Required:
  --namespace <ns>       Kubernetes namespace (staging|production|...)
  --previous-tag <tag>   Known-good image tag (git SHA). Forbidden: latest|dev|ci

Optional:
  --reason <text>        Free-form reason recorded in annotations
  --dry-run              Print actions only; do not call kubectl apply/set
  --skip-ready           Do not wait for /readyz after rollback
  --backend-only         Only roll backend
  --frontend-only        Only roll frontend
  --help                 Show this help

Examples:
  ./deploy/scripts/canary-rollback.sh \
    --namespace production \
    --previous-tag 0eb2373deadbeef \
    --reason canary-slo-breach

  READY_TIMEOUT_SEC=120 ./deploy/scripts/canary-rollback.sh \
    --namespace production \
    --previous-tag abcdef123456 \
    --dry-run
EOF
}

while [ $# -gt 0 ]; do
  case "$1" in
    --namespace) NAMESPACE="${2:-}"; shift 2 ;;
    --previous-tag) PREVIOUS_TAG="${2:-}"; shift 2 ;;
    --reason) REASON="${2:-}"; shift 2 ;;
    --dry-run) DRY_RUN=1; shift ;;
    --skip-ready) SKIP_READY=1; shift ;;
    --backend-only) BACKEND_ONLY=1; shift ;;
    --frontend-only) FRONTEND_ONLY=1; shift ;;
    --help|-h) usage; exit 0 ;;
    *) die "unknown argument: $1 (see --help)" ;;
  esac
done

[ -n "${NAMESPACE}" ] || die "--namespace is required"
[ -n "${PREVIOUS_TAG}" ] || die "--previous-tag is required"

case "${PREVIOUS_TAG}" in
  latest|dev|ci|0.0.0-unset|__IMAGE_TAG__|*placeholder*)
    die "previous-tag '${PREVIOUS_TAG}' is a forbidden mutable/placeholder tag"
    ;;
esac

if [ "${BACKEND_ONLY}" -eq 1 ] && [ "${FRONTEND_ONLY}" -eq 1 ]; then
  die "use only one of --backend-only / --frontend-only"
fi

DO_BACKEND=1
DO_FRONTEND=1
if [ "${BACKEND_ONLY}" -eq 1 ]; then DO_FRONTEND=0; fi
if [ "${FRONTEND_ONLY}" -eq 1 ]; then DO_BACKEND=0; fi

TS="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
log "canary-rollback start"
log "  namespace=${NAMESPACE}"
log "  previous_tag=${PREVIOUS_TAG}"
log "  reason=${REASON}"
log "  dry_run=${DRY_RUN}"
log "  backend=${DO_BACKEND} frontend=${DO_FRONTEND}"
log "  timestamp=${TS}"

run_kubectl() {
  if [ "${DRY_RUN}" -eq 1 ]; then
    log "[dry-run] kubectl $*"
    return 0
  fi
  kubectl "$@"
}

if [ "${DRY_RUN}" -eq 0 ]; then
  command -v kubectl >/dev/null 2>&1 || die "kubectl not found on PATH"
  kubectl cluster-info >/dev/null 2>&1 || fail2 "kubectl cannot reach cluster (check KUBECONFIG)"
fi

rollback_deploy() {
  local deploy="$1"
  local container="$2"
  local image="$3"
  local full="${image}:${PREVIOUS_TAG}"

  log "rollback ${deploy} container=${container} -> ${full}"
  run_kubectl -n "${NAMESPACE}" set image "deploy/${deploy}" "${container}=${full}"
  run_kubectl -n "${NAMESPACE}" annotate "deploy/${deploy}" \
    "animal-poke.io/last-rollback-at=${TS}" \
    "animal-poke.io/last-rollback-tag=${PREVIOUS_TAG}" \
    "animal-poke.io/last-rollback-reason=${REASON}" \
    --overwrite
  run_kubectl -n "${NAMESPACE}" annotate "deploy/${deploy}" \
    "animal-poke.io/canary-percent-" \
    "animal-poke.io/canary-image-" \
    --overwrite 2>/dev/null || true

  if [ "${DRY_RUN}" -eq 0 ]; then
    kubectl -n "${NAMESPACE}" rollout status "deploy/${deploy}" --timeout="${READY_TIMEOUT_SEC}s" \
      || fail2 "rollout status failed for ${deploy}"
  else
    log "[dry-run] would wait rollout status deploy/${deploy} --timeout=${READY_TIMEOUT_SEC}s"
  fi
}

if [ "${DO_BACKEND}" -eq 1 ]; then
  # container name in base backend.yaml is "backend"
  rollback_deploy "${BACKEND_DEPLOY}" "backend" "${BACKEND_IMAGE}"
fi
if [ "${DO_FRONTEND}" -eq 1 ]; then
  # container name in base frontend.yaml is "frontend"
  rollback_deploy "${FRONTEND_DEPLOY}" "frontend" "${FRONTEND_IMAGE}"
fi

check_readyz() {
  if [ "${SKIP_READY}" -eq 1 ]; then
    warn "skipping /readyz check (--skip-ready)"
    return 0
  fi
  if [ "${DRY_RUN}" -eq 1 ]; then
    log "[dry-run] would exec ${BACKEND_DEPLOY} wget /readyz and require HTTP 200"
    return 0
  fi
  if [ "${DO_BACKEND}" -ne 1 ]; then
    warn "backend not rolled; skip readyz"
    return 0
  fi

  local deadline=$((SECONDS + READY_TIMEOUT_SEC))
  local body code
  while [ "${SECONDS}" -lt "${deadline}" ]; do
    if body=$(kubectl -n "${NAMESPACE}" exec "deploy/${BACKEND_DEPLOY}" -- \
        wget -qO- http://127.0.0.1:8080/readyz 2>/dev/null); then
      log "readyz OK: ${body}"
      return 0
    fi
    # also try curl if wget missing in image (should not happen)
    if body=$(kubectl -n "${NAMESPACE}" exec "deploy/${BACKEND_DEPLOY}" -- \
        sh -c 'wget -qO- http://127.0.0.1:8080/readyz' 2>/dev/null); then
      log "readyz OK: ${body}"
      return 0
    fi
    sleep 3
  done
  fail3 "post-rollback /readyz did not become ready within ${READY_TIMEOUT_SEC}s"
}

check_readyz

log "canary-rollback SUCCESS"
log "  rolled back to tag=${PREVIOUS_TAG} reason=${REASON}"
log "  record: annotate animal-poke.io/last-rollback-* on deployments"
exit 0
