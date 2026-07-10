/**
 * AP-058 business-path load script.
 * Scenarios: auth → (optional) consent → geo → weak sync.
 * Vision path only when ENABLE_VISION=1 with FIXTURE_JPEG path.
 *
 * Business success rules (not generic http_req_failed):
 * - auth must be 200
 * - geo/weather: 200 OK; 503 counted as upstream_degraded (not business success)
 * - sync: 2xx = success; 4xx validation still "exercised" but not business success
 * - unexpected 5xx always fails business_success
 *
 * Usage:
 *   k6 run -e BASE_URL=http://localhost:8080 -e VUS=10 -e DURATION=1m deploy/loadtest/k6-core-loop.js
 */
import http from 'k6/http'
import { check, sleep } from 'k6'
import { Rate, Trend, Counter } from 'k6/metrics'
import { SharedArray } from 'k6/data'
import encoding from 'k6/encoding'

const BASE = __ENV.BASE_URL || 'http://localhost:8080'
const ENABLE_VISION = __ENV.ENABLE_VISION === '1'
const businessOK = new Rate('business_success')
const unexpected5xx = new Rate('unexpected_5xx')
const authLatency = new Trend('auth_ms')
const geoLatency = new Trend('geo_ms')
const visionLatency = new Trend('vision_ms')
const bizFails = new Counter('business_fail_total')

export const options = {
  vus: Number(__ENV.VUS || 10),
  duration: __ENV.DURATION || '1m',
  thresholds: {
    // Generic transport errors
    http_req_failed: ['rate<0.10'],
    http_req_duration: ['p(95)<800', 'p(99)<2000'],
    // Business path: auth must mostly succeed
    business_success: ['rate>0.90'],
    unexpected_5xx: ['rate<0.02'],
  },
}

function deviceId() {
  return `k6-core-${__VU}-${__ITER}-${Date.now()}`
}

function markBiz(ok, label) {
  businessOK.add(ok)
  if (!ok) bizFails.add(1, { step: label || 'unknown' })
}

export default function () {
  const ping = http.get(`${BASE}/livez`)
  if (ping.status !== 200) {
    unexpected5xx.add(true)
    markBiz(false, 'livez')
    sleep(1)
    return
  }

  const id = deviceId()
  const authRes = http.post(
    `${BASE}/api/v1/auth/device`,
    JSON.stringify({ device_id: id }),
    { headers: { 'Content-Type': 'application/json' } },
  )
  authLatency.add(authRes.timings.duration)
  const okAuth = authRes.status === 200
  markBiz(okAuth, 'auth')
  if (!okAuth) {
    if (authRes.status >= 500) unexpected5xx.add(true)
    sleep(1)
    return
  }
  unexpected5xx.add(false)

  let token = ''
  try {
    token = authRes.json('token') || ''
  } catch (_) {
    markBiz(false, 'auth_parse')
    return
  }
  if (!token) {
    markBiz(false, 'auth_token')
    return
  }

  const headers = {
    Authorization: `Bearer ${token}`,
    'Content-Type': 'application/json',
    'X-Request-ID': `k6-${__VU}-${__ITER}`,
  }

  const geo = http.get(`${BASE}/api/v1/geo/city?lat=29.87&lng=121.55`, { headers })
  geoLatency.add(geo.timings.duration)
  // 503 = degraded upstream, not a pass for business_success
  if (geo.status === 200) {
    markBiz(true, 'geo')
    unexpected5xx.add(false)
  } else if (geo.status === 503) {
    markBiz(false, 'geo_degraded')
    unexpected5xx.add(false)
  } else if (geo.status >= 500) {
    markBiz(false, 'geo_5xx')
    unexpected5xx.add(true)
  } else {
    markBiz(false, 'geo_4xx')
    unexpected5xx.add(false)
  }

  const weather = http.get(`${BASE}/api/v1/weather/week?lat=29.87&lng=121.55`, { headers })
  if (weather.status === 200) {
    markBiz(true, 'weather')
  } else if (weather.status === 503) {
    markBiz(false, 'weather_degraded')
  } else if (weather.status >= 500) {
    markBiz(false, 'weather_5xx')
    unexpected5xx.add(true)
  } else {
    markBiz(false, 'weather_other')
  }

  // Intentionally incomplete body: 4xx is expected without full payload.
  // Must NOT count 4xx as business success (AP-058).
  const sync = http.post(
    `${BASE}/api/v1/sync/animal`,
    JSON.stringify({ client_uuid: `uuid-${__VU}-${__ITER}` }),
    { headers: { ...headers, 'Idempotency-Key': `sync-${__VU}-${__ITER}` } },
  )
  if (sync.status >= 200 && sync.status < 300) {
    markBiz(true, 'sync')
  } else if (sync.status >= 500) {
    markBiz(false, 'sync_5xx')
    unexpected5xx.add(true)
  } else {
    // 4xx validation — exercised path, not business success
    markBiz(false, 'sync_validation')
  }

  if (ENABLE_VISION) {
    // Minimal 1x1 JPEG if no fixture (may 400); prefer FIXTURE_JPEG base64 env
    const b64 = __ENV.FIXTURE_JPEG_B64 || ''
    if (b64) {
      const bin = encoding.b64decode(b64)
      const res = http.post(`${BASE}/api/v1/vision/detect`, bin, {
        headers: {
          Authorization: `Bearer ${token}`,
          'Content-Type': 'image/jpeg',
          'X-Request-ID': `k6-vis-${__VU}-${__ITER}`,
          'Idempotency-Key': `vis-${__VU}-${__ITER}`,
        },
      })
      visionLatency.add(res.timings.duration)
      if (res.status >= 200 && res.status < 300) markBiz(true, 'vision')
      else if (res.status >= 500) {
        markBiz(false, 'vision_5xx')
        unexpected5xx.add(true)
      } else markBiz(false, 'vision_other')
    }
  }

  sleep(Number(__ENV.THINK || 0.3))
}
