# AP-111 Kubernetes reliability

## Controls

| Control | Location | Intent |
|---|---|---|
| PodDisruptionBudget | `deploy/k8s/base/pdb.yaml` | Keep ≥2 backend / ≥1 frontend during voluntary disruption |
| Topology spread + anti-affinity | `deploy/k8s/base/backend.yaml`, `frontend.yaml` | Cross-zone / cross-node placement |
| `terminationGracePeriodSeconds` + `preStop` | same | Drain: de-register before kill; finish in-flight writes |
| Production NetworkPolicy | `deploy/k8s/overlays/production/network-policy.yaml` | Default deny; allow DNS, deps, ingress, HTTPS APIs |
| Rolling strategy | backend/frontend Deployments | `maxUnavailable: 0` |

## Fault budgets

See generated table from:

```bash
./deploy/scripts/reliability-drain-drill.sh --assert-only --record /tmp/ap111-budget.md
```

## Validation

```bash
IMAGE_TAG=$GITHUB_SHA ./deploy/k8s/scripts/assert-manifests.sh
./deploy/scripts/reliability-drain-drill.sh --assert-only
# after apply to staging/production:
./deploy/scripts/reliability-drain-drill.sh --live-check
```

## Rollback

1. Revert the overlay commit or set previous image tag via release pipeline.
2. `kubectl -n production rollout undo deploy/animal-poke-backend`
3. NetworkPolicy tightens egress — if a required dependency is blocked, temporarily
   widen `production-backend-allow` egress ports, then fix the dependency endpoint.
