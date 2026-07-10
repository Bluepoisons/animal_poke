#!/usr/bin/env bash
# vision-golden-stub: PR contract path for AP-047 ML golden set.
# Runs stub provider only — no real Vision API keys required.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

echo "==> vision golden stub (AP-047)"
go test ./internal/mlqa/ -count=1 -timeout=60s "$@"
echo "==> OK"
