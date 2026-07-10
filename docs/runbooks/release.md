# Release runbook — staging → canary → production (AP-039)

This document describes the Animal Poke release pipeline, gates, smoke hooks,
and rollback. The executable workflow is `.github/workflows/release.yml`.

## Goals

- Immutable images tagged with **git commit SHA** (never `latest` / `dev` / `ci`).
- Progressive path: **build → kustomize dry-run → staging smoke → canary (manual approval) → production**.
- Hard readiness: `/readyz` non-200 **aborts** the release (no soft-fail / `|| true`).
- One-click rollback via `deploy/scripts/canary-rollback.sh`.
- Release record: git SHA, image digests, schema/config notes, rollback command.

## Artifacts

| Artifact | Path / location |
| --- | --- |
| Release workflow | `.github/workflows/release.yml` (`workflow_dispatch`) |
| CI container smoke | `.github/workflows/ci.yml` (livez + **hard** readyz) |
| Kustomize overlays | `deploy/k8s/overlays/{staging,production}` |
| Manifest assertions | `deploy/k8s/scripts/assert-manifests.sh` |
| Frontend smoke | `deploy/scripts/frontend-smoke.sh` |
| Canary / prod rollback | `deploy/scripts/canary-rollback.sh` |
| Schema migrate | `docs/runbooks/schema-migrate.md` |
| Frontend release notes | `docs/runbooks/frontend-release.md` |

## Image tags

```text
registry.cn-beijing.aliyuncs.com/animal-poke/backend:<git-sha>
registry.cn-beijing.aliyuncs.com/animal-poke/frontend:<git-sha>
```

Inject into overlays:

```bash
IMAGE_TAG=<git-sha>
work=$(mktemp -d)
cp -a deploy/k8s/. "$work/k8s/"
(
  cd "$work/k8s/overlays/staging"   # or production
  kustomize edit set image \
    "registry.cn-beijing.aliyuncs.com/animal-poke/backend:${IMAGE_TAG}" \
    "registry.cn-beijing.aliyuncs.com/animal-poke/frontend:${IMAGE_TAG}"
  kustomize build .
)
```

CI and release both call `deploy/k8s/scripts/assert-manifests.sh` with `IMAGE_TAG`.

## Workflow dispatch inputs

| Input | Meaning |
| --- | --- |
| `git_ref` | Branch / tag / SHA to release (default: triggering SHA) |
| `image_tag` | Override tag (default: full commit SHA) |
| `skip_push` | Build only; skip registry push (default **true** for safe dry validation) |
| `deploy_staging` | Apply staging when `KUBECONFIG_STAGING` secret exists |
| `canary_approval` | Must be exactly `APPROVE_CANARY` to open canary/production |
| `canary_percent` | 1–50 (default 10) |
| `promote_production` | Promote full production after canary |
| `dry_run_only` | Default **true**: build + dry-run + hooks only, no apply |

### Safe first run (recommended)

1. Actions → **Release** → Run workflow
2. Leave `skip_push=true`, `dry_run_only=true`
3. Leave `canary_approval` empty (or anything other than `APPROVE_CANARY`)
4. Confirm: build, local smoke (livez+readyz), kustomize assert, rollback dry-run, staging hooks

### Promote path

1. Re-run with `skip_push=false` (requires `REGISTRY_USERNAME` / `REGISTRY_PASSWORD`)
2. Optional: `deploy_staging=true` + `dry_run_only=false` + `KUBECONFIG_STAGING`
3. After staging green: re-run with `canary_approval=APPROVE_CANARY`, `dry_run_only=false`
4. Production: also set `promote_production=true` (+ `KUBECONFIG_PRODUCTION`)

## Pipeline stages

```text
resolve metadata
    │
    ▼
build images (SHA tags) + local livez/readyz + frontend-smoke
    │
    ▼
kustomize dry-run (staging + production) + assert-manifests + rollback --dry-run
    │
    ▼
staging smoke hooks
    │  online if STAGING_API_URL / STAGING_APP_URL vars set
    │  optional kubectl apply if deploy_staging + kubeconfig
    ▼
canary gate  ── requires canary_approval == APPROVE_CANARY
    │
    ▼
canary deploy (optional kube) + SLO abort → canary-rollback.sh
    │
    ▼
production promote (optional kube) + release record
```

## Staging smoke hooks

When cluster URLs are configured (`vars.STAGING_API_URL`, `vars.STAGING_APP_URL`):

1. Pre-deploy migrate Job (`animal-poke-migrate up`) — see `schema-migrate.md`
2. Apply staging overlay with `IMAGE_TAG`
3. Wait Deployments ready
4. **`GET /livez` and `GET /readyz` must be 200** (hard fail)
5. `./deploy/scripts/frontend-smoke.sh` against app host
6. Contract smoke on critical OpenAPI paths (auth / health)
7. Optional: `deploy/loadtest/k6-smoke.js`
8. Optional: DAST baseline (ZAP) if org tooling is wired

Without online URLs, the workflow still validates:

- local container smoke (including **hard readyz**)
- kustomize isolation / immutable tags
- rollback script dry-run

## Canary approval

There is **no silent promote**. Canary and production jobs require:

```text
canary_approval = APPROVE_CANARY
```

Invalid approval:

- stops after staging smoke when `promote_production=false`
- **fails the workflow** when `promote_production=true` without approval

## Auto-rollback criteria

Wire these to your metrics backend (Prometheus / APM). On breach, run:

```bash
./deploy/scripts/canary-rollback.sh \
  --namespace production \
  --previous-tag <last-known-good-sha> \
  --reason canary-slo-breach
```

Suggested abort rules:

| Signal | Threshold (starting point) |
| --- | --- |
| HTTP 5xx rate | > 2% over 5 minutes on canary pods |
| `/readyz` | any non-200 on canary backend |
| Core funnel (auth → detect → sync) | success < 95% |
| P95 latency | > 2× pre-canary baseline |

The release workflow also invokes rollback automatically when post-canary
`/readyz` fails and `vars.PRODUCTION_PREVIOUS_IMAGE_TAG` is set.

## Manual one-click rollback

```bash
# Inspect current images
kubectl -n production get deploy animal-poke-backend animal-poke-frontend -o wide

# Roll both Deployments to previous SHA
./deploy/scripts/canary-rollback.sh \
  --namespace production \
  --previous-tag <previous-git-sha> \
  --reason manual-rollback

# Dry-run only
./deploy/scripts/canary-rollback.sh \
  --namespace production \
  --previous-tag <previous-git-sha> \
  --dry-run
```

The script:

1. Rejects mutable tags (`latest` / `dev` / `ci` / placeholders)
2. `kubectl set image` backend + frontend to `*:previous-tag`
3. Annotates `animal-poke.io/last-rollback-*` and clears canary annotations
4. Waits for rollout
5. **Hard-checks `/readyz`** (unless `--skip-ready` / `--dry-run`)

## Release record checklist

Every production release should record:

- [ ] `git_sha` (full)
- [ ] `image_tag` (usually same as sha)
- [ ] backend + frontend **registry digests**
- [ ] DB schema version (`animal-poke-migrate status`)
- [ ] Config / feature-flag version notes
- [ ] Canary percent + approval actor
- [ ] Rollback command with previous tag

GitHub Actions writes a subset of this to the job **Summary**.

## Secrets & variables

| Name | Type | Purpose |
| --- | --- | --- |
| `REGISTRY_USERNAME` / `REGISTRY_PASSWORD` | secret | Push images when `skip_push=false` |
| `KUBECONFIG_STAGING` | secret | base64 kubeconfig for staging apply |
| `KUBECONFIG_PRODUCTION` | secret | base64 kubeconfig for canary/prod |
| `COSIGN_PRIVATE_KEY` / `COSIGN_PASSWORD` | secret | Optional image signing |
| `STAGING_API_URL` | variable | e.g. `https://api.staging.animal-poke.example.com` |
| `STAGING_APP_URL` | variable | e.g. `https://app.staging.animal-poke.example.com` |
| `PRODUCTION_PREVIOUS_IMAGE_TAG` | variable | Last known-good SHA for auto-rollback |

Do **not** commit kubeconfigs, registry passwords, or cosign keys.

## CI hard readyz (related fix)

Container job in `ci.yml` must:

1. Wait for `/livez`
2. Wait for `/readyz` with **HTTP 200** (no `|| true` on the assertion)
3. Fail the job if readyz never becomes ready

Development CI runs backend with `APP_ENV=development` so missing DB does not
false-fail readiness; production/staging readiness still requires DB up.

## Schema compatibility

- Prefer **expand/contract** migrations (see `schema-migrate.md`).
- Canary/production rollback of **application** images must remain compatible
  with the expanded schema.
- Never roll back a destructive contract migration without a data restore plan
  (`docs/runbooks/backup-and-dr.md`).

## Failure playbooks

### readyz non-200 after deploy

1. `kubectl -n <ns> logs deploy/animal-poke-backend --tail=200`
2. Check DB connectivity / migrate Job completion
3. If canary: run `canary-rollback.sh` immediately
4. Do **not** promote production

### Staging smoke fails

1. Do not approve canary
2. Fix forward on a new SHA; re-run Release from the fixed ref

### Registry push fails

1. Keep `skip_push=true` path green for validation
2. Fix credentials / network; never retag to `latest` as a workaround

## Related docs

- `docs/runbooks/frontend-release.md` — frontend image / CSP / smoke details
- `docs/runbooks/schema-migrate.md` — migrate Job and expand/contract
- `docs/runbooks/backup-and-dr.md` — backup before risky deploys
- `docs/runbooks/secret-rotation.md` — post-rotation readiness checks
