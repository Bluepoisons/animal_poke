#!/bin/sh
# Generate runtime public config for the SPA (same-origin /api by default).
# Runs under official nginx docker-entrypoint.d before nginx starts.
# Non-root (K8s) may not write html/; baked default config.js is used then.
set -eu

HTML_ROOT="${HTML_ROOT:-/usr/share/nginx/html}"
API_BASE_URL="${API_BASE_URL:-}"
CONFIG_JS="${HTML_ROOT}/config.js"

# Validate optional absolute API base URL; empty means same-origin relative /api
if [ -n "${API_BASE_URL}" ]; then
  case "${API_BASE_URL}" in
    http://*|https://*|/*)
      API_BASE_URL=$(printf '%s' "${API_BASE_URL}" | sed 's:/*$::')
      ;;
    *)
      echo "ERROR: API_BASE_URL must be empty, start with /, or be an absolute http(s) URL (got: ${API_BASE_URL})" >&2
      exit 1
      ;;
  esac
fi

# If we cannot write (non-root), keep image default when API_BASE_URL is empty
if [ ! -w "${HTML_ROOT}" ] && [ ! -w "${CONFIG_JS}" ]; then
  if [ -n "${API_BASE_URL}" ]; then
    echo "ERROR: cannot write ${CONFIG_JS} as non-root but API_BASE_URL is set" >&2
    exit 1
  fi
  echo "WARN: ${HTML_ROOT} not writable; using baked config.js (same-origin /api)"
  exit 0
fi

js_escape() {
  printf '%s' "$1" | sed "s/\\\\/\\\\\\\\/g; s/'/\\\\'/g"
}

ESCAPED=$(js_escape "${API_BASE_URL}")

cat > "${CONFIG_JS}" <<EOF
/* generated at container start — public config only */
window.__AP_CONFIG__ = Object.freeze({
  apiBaseUrl: '${ESCAPED}'
});
EOF

echo "Wrote ${CONFIG_JS} (apiBaseUrl='${API_BASE_URL:-<same-origin>}')"
