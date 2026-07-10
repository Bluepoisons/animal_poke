# SLO, dashboards, and alerts (AP-035)

## Correlation fields (safe)
- `request_id` (HTTP middleware)
- `release` (`app_info{release=...}` / `RELEASE_SHA`)
- AI inference id in logs (never photos / precise coordinates)

## Core SLOs
| SLO | Target | Signal |
|-----|--------|--------|
| Core loop success (detect→capture→sync) | ≥ 99% / 30d | `game_funnel_total` |
| Vision detect P95 | ≤ 3s | HTTP RED on `/api/v1/vision/detect` |
| Sync accept latency P95 | ≤ 1s | `/api/v1/sync/*` |
| Error rate 5xx | ≤ 0.5% | `http_requests_total{status="5xx"}` |
| Empty detect surge | alert if rate > 5× baseline 10m | `ai_detect_empty_total` |
| AI cost growth | budget burn < 120% day | `ai_calls_total` |

## Scrape
- Management port only: `METRICS_ADDR` default `:9090` Service `animal-poke-backend-metrics`
- Not on public Ingress (AP-036)

## Alerts (examples)
See `deploy/observability/alerts.example.yaml`.

## Runbook: provider 5xx / empty surge
1. Confirm `app_info{release=...}` and spike window.
2. Check `ai_calls_total{outcome="error"}` by type/provider.
3. Disable feature flag / raise rate limit if abuse.
4. Follow vision provider status; enable mock only in non-prod.

## Runbook: DB down
1. `readyz` failing while `livez` ok.
2. Check DB pool / network policy.
3. Page on-call; avoid deploy until ready.

## Verify
```bash
curl -sS localhost:9090/metrics | head
go test ./internal/middleware -count=1
```
