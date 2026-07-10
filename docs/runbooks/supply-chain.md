# Supply Chain Security Runbook (AP-041)

## Package manager (frontend)

- **Single package manager: npm** (Corepack / pnpm / yarn are not used).
- Canonical lockfile: `frontend/package-lock.json` (committed).
- `frontend/package.json` declares:
  - `packageManager`: `npm@10.9.2` (document preferred npm major/minor)
  - `engines.node`: `>=22 <23` (Node 22 LTS only)
  - `engines.npm`: `>=10`
- `frontend/.npmrc` sets `engine-strict=true` so mismatched Node/npm fail install.
- `frontend/.nvmrc` pins Node major `22` for nvm / fnm / asdf.
- Install locally and in CI with:

```bash
cd frontend
npm ci
```

Do **not** reintroduce `pnpm-lock.yaml` / `yarn.lock`. If a second lockfile appears, delete it and re-run `npm install` so only `package-lock.json` remains.

## Node / engines gate

- Preinstall script `frontend/scripts/check-node.mjs` rejects non-22 majors.
- Docker builder image: `node:22.14.0-alpine` (see `deploy/Dockerfile.frontend`).
- CI uses `NODE_VERSION: "22"` via `actions/setup-node`.

## CI controls

Workflow: `.github/workflows/ci.yml`

| Control | Location | Behavior |
|--------|----------|----------|
| Deterministic install | Frontend job | `npm ci` only (no `npm install` fallback) |
| Vulnerability gate | Frontend job | `npm audit --audit-level=high` (fails on high/critical) |
| SBOM | Frontend job | `npm sbom` CycloneDX → artifact `frontend-sbom` |
| Go vulns | Backend job | `govulncheck` **pinned** module version (never `@latest`) |
| Static analysis | Backend job | `staticcheck@2026.1` pinned |
| Secrets | `secret-scan` job | **gitleaks** via `gitleaks/gitleaks-action@v3` + `.gitleaks.toml` |

### Regenerating SBOM locally

```bash
cd frontend
npm ci
npm run sbom   # writes sbom.cdx.json (gitignored if present locally)
```

CI uploads `frontend/sbom.cdx.json` as a workflow artifact for release / audit evidence.

## Image / artifact signing (cosign) — planned

**Status:** not yet enforced in CI. Track as follow-up hardening.

Recommended adoption path:

1. Generate a cosign key pair in CI secrets (or use keyless OIDC with GitHub Actions + Sigstore).
2. After `docker build` of backend/frontend images, run:

   ```bash
   cosign sign --yes "$IMAGE_REF"
   cosign verify "$IMAGE_REF"
   ```

3. Prefer **keyless** signing (`cosign sign` with GitHub OIDC) so no long-lived private key lives in the repo.
4. Gate production deploy on `cosign verify` + immutable image tag (`$GITHUB_SHA`), matching K8s overlay policy.
5. Optionally attach SBOM as a cosign attestation:

   ```bash
   cosign attest --predicate frontend/sbom.cdx.json --type cyclonedx "$IMAGE_REF"
   ```

Until cosign is wired, production still requires **immutable tags** (no `:latest` / `:dev`) per K8s assert scripts.

## Secret scanning

- **gitleaks** already runs on every PR/push (`secret-scan` job).
- Config: `.gitleaks.toml`.
- On finding: rotate keys per `docs/runbooks/secret-rotation.md`, purge history if needed, re-run gitleaks.

## Dependency updates

1. Change versions only via `package.json` + `npm install` inside `frontend/` (refreshes `package-lock.json`).
2. Run `npm audit --audit-level=high` and fix or document accepted risk before merge.
3. Backend: `go get` + `go mod tidy`; re-run pinned `govulncheck`.
4. Never commit `node_modules/`, vendor secrets, or alternate lockfiles.

## Checklist for supply-chain PRs

- [ ] Only `frontend/package-lock.json` (no pnpm/yarn lock)
- [ ] `packageManager` + `engines` still accurate
- [ ] CI still uses `npm ci` + audit + SBOM artifact
- [ ] No `@latest` pins for security tools in workflows
- [ ] gitleaks clean
- [ ] (Future) cosign sign/verify for release images
