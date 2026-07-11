#!/usr/bin/env bash
# AP-111 reliability drill: voluntary disruption budget check (dry-run by default).
#
# Does NOT require a live cluster when run with --assert-only (CI/local gate).
# Live modes need kubectl context + production namespace.
#
# Usage:
#   ./deploy/scripts/reliability-drain-drill.sh --assert-only
#   ./deploy/scripts/reliability-drain-drill.sh --live-check   # kubectl dry-run drain
#   ./deploy/scripts/reliability-drain-drill.sh --record /tmp/drill.md
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MODE="assert-only"
RECORD=""
NS="${NAMESPACE:-production}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --assert-only) MODE="assert-only"; shift ;;
    --live-check) MODE="live-check"; shift ;;
    --record) RECORD="${2:-}"; shift 2 ;;
    *) echo "unknown arg: $1" >&2; exit 2 ;;
  esac
done

fail() { echo "DRILL FAIL: $*" >&2; exit 1; }
pass() { echo "DRILL OK: $*"; }

assert_manifests_have_reliability() {
  local tag="${IMAGE_TAG:-deadbeefcafebabe0123456789abcdef01234567}"
  local prod
  prod="$(mktemp)"
  IMAGE_TAG="$tag" "${ROOT}/deploy/k8s/scripts/assert-manifests.sh" >/dev/null
  # Build again for local pattern checks (assert-manifests already validates)
  local work
  work="$(mktemp -d)"
  cp -a "${ROOT}/deploy/k8s/." "${work}/k8s/"
  (
    cd "${work}/k8s/overlays/production"
    kustomize edit set image \
      "registry.cn-beijing.aliyuncs.com/animal-poke/backend:${tag}" \
      "registry.cn-beijing.aliyuncs.com/animal-poke/frontend:${tag}"
    kustomize build . > "${prod}"
  )
  rm -rf "${work}"

  grep -q 'kind: PodDisruptionBudget' "${prod}" || fail "PDB missing in production render"
  grep -q 'animal-poke-backend-pdb' "${prod}" || fail "backend PDB missing"
  grep -q 'topologySpreadConstraints' "${prod}" || fail "topology spread missing"
  grep -q 'topology.kubernetes.io/zone' "${prod}" || fail "zone topology key missing"
  grep -q 'preStop' "${prod}" || fail "preStop lifecycle missing"
  grep -q 'terminationGracePeriodSeconds' "${prod}" || fail "terminationGracePeriodSeconds missing"
  grep -q 'kind: NetworkPolicy' "${prod}" || fail "production NetworkPolicy missing"
  grep -q 'production-backend-allow' "${prod}" || fail "backend NetworkPolicy missing"
  pass "rendered production manifests include PDB/spread/preStop/NetworkPolicy"
  rm -f "${prod}"
}

live_check() {
  command -v kubectl >/dev/null || fail "kubectl not installed"
  kubectl -n "${NS}" get pdb animal-poke-backend-pdb >/dev/null \
    || fail "backend PDB not applied in ${NS}"
  kubectl -n "${NS}" get networkpolicy production-backend-allow >/dev/null \
    || fail "backend NetworkPolicy not applied"
  # Dry-run drain of first backend node (does not actually drain).
  local node
  node="$(kubectl -n "${NS}" get pod -l app=animal-poke-backend -o jsonpath='{.items[0].spec.nodeName}' 2>/dev/null || true)"
  if [[ -z "${node}" ]]; then
    pass "no running backend pods — skip dry-run drain"
    return 0
  fi
  kubectl drain "${node}" --dry-run=client --ignore-daemonsets --delete-emptydir-data \
    || fail "dry-run drain failed for ${node}"
  pass "dry-run drain accepted for node ${node} (PDB will block excess voluntary eviction)"
}

record_budget() {
  local out="$1"
  cat >"${out}" <<'EOF'
# AP-111 reliability budget (static plan)

| Scenario | Budget | Expected recovery |
|---|---|---|
| Single node drain | Keep ≥2 backend ready (PDB minAvailable=2) | < 60s after drain starts (preStop 15s + reschedule) |
| Single zone loss | HPA minReplicas=3 + zone topology spread | Remaining zones serve traffic; scale-up ≤ 2m |
| Rolling upgrade | maxUnavailable=0, maxSurge=1 | Zero planned downtime for /readyz |
| Redis/MySQL blip | Readiness fails; LB stops new traffic | No double-charge: wallet/outbox idempotent keys |
| Canary rollback | `deploy/scripts/canary-rollback.sh` | < 5m to previous image tag |

Live chaos (pod kill / DNS inject) is **not** executed in CI; run under staging with
`./deploy/scripts/reliability-drain-drill.sh --live-check` after apply.
EOF
  pass "wrote budget to ${out}"
}

case "${MODE}" in
  assert-only) assert_manifests_have_reliability ;;
  live-check)
    assert_manifests_have_reliability
    live_check
    ;;
  *) fail "unknown mode ${MODE}" ;;
esac

if [[ -n "${RECORD}" ]]; then
  record_budget "${RECORD}"
fi
