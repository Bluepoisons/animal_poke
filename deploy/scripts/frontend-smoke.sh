#!/usr/bin/env bash
# Frontend image smoke checks (local + CI).
# Expects a running frontend base URL (default http://127.0.0.1:18081).
# Optional: backend reachable via frontend /api reverse proxy.
set -euo pipefail

BASE_URL="${1:-${FRONTEND_SMOKE_URL:-http://127.0.0.1:18081}}"
STRICT_API="${STRICT_API:-1}"

echo "==> Frontend smoke against ${BASE_URL}"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

assert_http() {
  local path="$1"
  local want_code="${2:-200}"
  local code
  code=$(curl -sS -o /tmp/ap-smoke-body -w '%{http_code}' "${BASE_URL}${path}" || true)
  if [ "${code}" != "${want_code}" ]; then
    echo "body:" >&2
    head -c 400 /tmp/ap-smoke-body >&2 || true
    echo >&2
    fail "${path} expected HTTP ${want_code}, got ${code}"
  fi
  echo "OK  ${path} -> ${code}"
}

assert_not_html_body() {
  local path="$1"
  local label="${2:-$1}"
  local body
  body=$(head -c 400 /tmp/ap-smoke-body || true)
  if echo "${body}" | grep -qiE '<!DOCTYPE html|<html'; then
    fail "${label} body looks like HTML index (got: ${body})"
  fi
}

# 1) health
assert_http /healthz 200
grep -qx 'ok' /tmp/ap-smoke-body || fail "/healthz body must be 'ok'"

# 2) index
assert_http /index.html 200
grep -qi 'id="root"' /tmp/ap-smoke-body || fail "index.html missing #root"
cp /tmp/ap-smoke-body /tmp/ap-smoke-index

# 3) runtime config
assert_http /config.js 200
grep -q '__AP_CONFIG__' /tmp/ap-smoke-body || fail "config.js missing __AP_CONFIG__"

# 4) hashed assets referenced by index
asset=$(grep -Eo '/assets/[^"'\''[:space:]]+' /tmp/ap-smoke-index | head -1 || true)
if [ -n "${asset}" ]; then
  assert_http "${asset}" 200
else
  echo "WARN: no /assets/ reference found in index.html"
fi

# 5) PWA manifest / SW if present (soft if 404)
for path in /manifest.webmanifest /manifest.json /sw.js /registerSW.js; do
  code=$(curl -sS -o /tmp/ap-smoke-body -w '%{http_code}' "${BASE_URL}${path}" || true)
  if [ "${code}" = "200" ]; then
    assert_not_html_body "${path}"
    echo "OK  ${path} -> 200"
  else
    echo "SKIP ${path} -> ${code} (optional)"
  fi
done

# 6) /api must never return HTML SPA shell
API_PATH="${API_SMOKE_PATH:-/api/v1/auth/device}"
code=$(curl -sS -D /tmp/ap-smoke-headers -o /tmp/ap-smoke-body -w '%{http_code}' \
  -X POST "${BASE_URL}${API_PATH}" \
  -H 'Content-Type: application/json' \
  -d '{"device_id":"smoke-device-001"}' || true)
ct=$(grep -i '^content-type:' /tmp/ap-smoke-headers | head -1 | tr -d '\r' || true)
body=$(head -c 300 /tmp/ap-smoke-body || true)
echo "API ${API_PATH} -> HTTP ${code} ${ct}"
if [ "${code}" = "000" ]; then
  fail "API request failed to connect"
fi
if echo "${ct}" | grep -qi 'text/html'; then
  fail "API returned text/html — nginx SPA fallback is incorrectly handling /api"
fi
assert_not_html_body "${API_PATH}" "API ${API_PATH}"
if [ "${STRICT_API}" = "1" ]; then
  # With backend up: JSON 2xx/4xx; with backend down: 502 JSON from nginx
  if [ -n "${ct}" ] && ! echo "${ct}" | grep -qi 'json\|plain'; then
    echo "WARN: unexpected content-type ${ct} (body: ${body})"
  fi
fi
echo "OK  API path returns non-HTML"

# 7) GET API path non-HTML
for path in /api/v1/time /api/v1/ping; do
  code=$(curl -sS -D /tmp/ap-smoke-headers -o /tmp/ap-smoke-body -w '%{http_code}' "${BASE_URL}${path}" || true)
  ct=$(grep -i '^content-type:' /tmp/ap-smoke-headers | head -1 | tr -d '\r' || true)
  if echo "$(head -c 200 /tmp/ap-smoke-body || true)" | grep -qiE '<!DOCTYPE html|<html'; then
    fail "${path} returned HTML"
  fi
  if echo "${ct}" | grep -qi 'text/html'; then
    fail "${path} content-type is HTML"
  fi
  echo "OK  ${path} -> ${code} (non-HTML)"
  break
done

echo "==> Frontend smoke PASSED"
