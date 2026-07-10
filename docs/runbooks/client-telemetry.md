# Client error telemetry & private source maps (AP-037)

## Schema
`POST /api/v1/errors/report` wire fields (strict):
- message, stack, component, route, release, level, request_id, extra

Client maps richer `ErrorReport` → `ErrorReportPayload` via `toWirePayload()`.

## Privacy
- Client + server redact tokens, JWTs, data:image, lat/lng
- Never attach photos or precise coordinates

## Release / source maps
- Build with `VITE_RELEASE=$GITHUB_SHA` (injected as `__RELEASE_SHA__`)
- Vite `build.sourcemap: 'hidden'` generates maps for private upload without browser references
- Frontend image strips `*.map` from public nginx root
- Upload maps to private error platform (Sentry/etc.) out-of-band; not via public CDN

## Dedup / sampling / offline
- Client fingerprint dedup window
- Sample non-fatal noise
- Offline queue flushes on reconnect

## Verify
```bash
cd frontend && npm test -- src/errors
cd backend && go test ./internal/handlers -count=1
```
