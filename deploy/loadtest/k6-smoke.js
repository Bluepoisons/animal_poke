/**
 * k6 smoke/capacity script for animal_poke backend.
 * Usage:
 *   k6 run -e BASE_URL=http://localhost:8080 -e VUS=20 -e DURATION=2m deploy/loadtest/k6-smoke.js
 *
 * Scenarios cover: ping, auth, geo, weather, sync (stub body).
 * Vision is optional via ENABLE_VISION=1 with a tiny JPEG fixture.
 */
import http from 'k6/http'
import { check, sleep } from 'k6'
import { Rate, Trend } from 'k6/metrics'

const BASE = __ENV.BASE_URL || 'http://localhost:8080'
const errorRate = new Rate('errors')
const authLatency = new Trend('auth_ms')
const geoLatency = new Trend('geo_ms')

export const options = {
  vus: Number(__ENV.VUS || 10),
  duration: __ENV.DURATION || '1m',
  thresholds: {
    http_req_failed: ['rate<0.05'],
    http_req_duration: ['p(95)<800', 'p(99)<2000'],
    errors: ['rate<0.05'],
  },
}

function deviceId() {
  return `k6-${__VU}-${__ITER}-${Date.now()}`
}

export default function () {
  const ping = http.get(`${BASE}/livez`)
  check(ping, { 'livez 200': (r) => r.status === 200 })

  const id = deviceId()
  const authRes = http.post(
    `${BASE}/api/v1/auth/device`,
    JSON.stringify({ device_id: id }),
    { headers: { 'Content-Type': 'application/json' } },
  )
  authLatency.add(authRes.timings.duration)
  const okAuth = check(authRes, { 'auth 200': (r) => r.status === 200 })
  errorRate.add(!okAuth)
  if (!okAuth) {
    sleep(1)
    return
  }
  const token = authRes.json('token')
  const headers = {
    Authorization: `Bearer ${token}`,
    'Content-Type': 'application/json',
    'X-Request-ID': `k6-${__VU}-${__ITER}`,
  }

  const geo = http.get(`${BASE}/api/v1/geo/city?lat=29.87&lng=121.55`, { headers })
  geoLatency.add(geo.timings.duration)
  errorRate.add(!(geo.status === 200 || geo.status === 503))

  const weather = http.get(`${BASE}/api/v1/weather/week?lat=29.87&lng=121.55`, { headers })
  errorRate.add(!(weather.status === 200 || weather.status === 503))

  // lightweight sync attempt (may 400 without full payload — still exercises auth path)
  const sync = http.post(
    `${BASE}/api/v1/sync/animal`,
    JSON.stringify({ client_uuid: `uuid-${__VU}-${__ITER}` }),
    { headers: { ...headers, 'Idempotency-Key': `sync-${__VU}-${__ITER}` } },
  )
  errorRate.add(sync.status >= 500)

  sleep(Number(__ENV.THINK || 0.3))
}
