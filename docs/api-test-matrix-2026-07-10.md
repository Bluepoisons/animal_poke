# Deprecated — see AP-091 matrix

This snapshot claimed **35** endpoints and drifted from OpenAPI.

**Canonical sources (AP-091):**

- [`docs/api-test-matrix.md`](./api-test-matrix.md) — human-readable matrix (49 operations)
- [`docs/api-operations.inventory.json`](./api-operations.inventory.json) — generated inventory
- [`docs/api-test-matrix.json`](./api-test-matrix.json) — success/failure test refs
- `node scripts/api-test-matrix-gate.mjs` — CI gate
- `go test ./internal/routes -run TestContractMatrix` — per-operation success+failure smoke

Regenerate:

```bash
node scripts/api-test-matrix-gate.mjs --write
```
