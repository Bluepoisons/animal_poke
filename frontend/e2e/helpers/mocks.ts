declare global {
  interface Window {
    __AP_FORCE_CAMERA_READY?: boolean
    __AP_FORCE_CAPTURE_SUCCESS?: boolean
  }
}

import type { Page, Route } from '@playwright/test'

/** Call counters for hard-gate assertions */
export type ApiCallLog = {
  auth: number
  consent: number
  detect: number
  analyze: number
  value: number
  sync: number
  pull: number
  geo: number
  weather: number
  other: string[]
}

export function createApiCallLog(): ApiCallLog {
  return {
    auth: 0,
    consent: 0,
    detect: 0,
    analyze: 0,
    value: 0,
    sync: 0,
    pull: 0,
    geo: 0,
    weather: 0,
    other: [],
  }
}

async function json(route: Route, status: number, body: unknown, extraHeaders?: Record<string, string>) {
  await route.fulfill({
    status,
    contentType: 'application/json',
    headers: {
      'X-Request-ID': `e2e-${Date.now()}`,
      ...extraHeaders,
    },
    body: JSON.stringify(body),
  })
}

/**
 * Install deterministic backend stubs for the full capture loop.
 * Paths match production frontend services (auth/consent/detect/analyze/value/sync).
 */
export async function installApiMocks(
  page: Page,
  log: ApiCallLog,
  opts?: { failDetectOnce?: boolean; failSyncOnce?: boolean },
): Promise<void> {
  let detectFailed = false
  let syncFailed = false

  await page.route(/\/api\/v1\//, async (route) => {
    const req = route.request()
    const url = new URL(req.url())
    const path = url.pathname
    const method = req.method().toUpperCase()

    if (path.includes('/auth/device') && method === 'POST') {
      log.auth += 1
      return json(route, 200, {
        token: 'e2e-test-token',
        expires_at: new Date(Date.now() + 3600_000).toISOString(),
        token_type: 'Bearer',
      })
    }

    if (path.includes('/privacy/consent') && method === 'POST') {
      log.consent += 1
      return json(route, 200, { ok: true, version: 'v1' })
    }

    if (path.includes('/vision/detect') && method === 'POST') {
      log.detect += 1
      if (opts?.failDetectOnce && !detectFailed) {
        detectFailed = true
        return json(route, 500, { error: 'detect_stub_fail', reason_code: 'upstream' })
      }
      return json(route, 200, {
        inference_id: 'inf-e2e-cat-1',
        animals: [
          {
            species: 'cat',
            label: 'cat',
            target_id: '0',
            confidence: 0.94,
            bounding_box: { x: 0.2, y: 0.2, w: 0.4, h: 0.4 },
          },
        ],
        targets: [
          {
            species: 'cat',
            label: 'cat',
            target_id: '0',
            confidence: 0.94,
            bounding_box: { x: 0.2, y: 0.2, w: 0.4, h: 0.4 },
          },
        ],
        source: 'e2e-stub',
      })
    }

    if (path.includes('/vision/analyze') && method === 'POST') {
      log.analyze += 1
      return json(route, 200, {
        breed: 'Tabby',
        color: 'orange',
        body_type: 'normal',
        quality_score: 8,
        subject_completeness: 9,
        clarity: 8,
        lighting: 7,
        composition: 8,
        pose: 7,
        angle: 6,
        inference_id: 'inf-e2e-analyze-1',
        species: 'cat',
        target_id: '0',
      })
    }

    if (path.includes('/value/generate') && method === 'POST') {
      log.value += 1
      return json(route, 200, {
        rarity: 2,
        hp: 55,
        atk: 16,
        def: 14,
        spd: 20,
        class: 'Ranger',
        element: 'Wind',
        narrative: 'E2E stub companion found near the test park.',
        inference_id: 'inf-e2e-value-1',
      })
    }

    if (path.includes('/sync/animal') && method === 'POST') {
      log.sync += 1
      if (opts?.failSyncOnce && !syncFailed) {
        syncFailed = true
        return json(route, 503, { error: 'sync_unavailable', reason_code: 'temporary' })
      }
      return json(route, 200, { ok: true, status: 'synced' })
    }

    if (path.includes('/sync/animals') && method === 'GET') {
      log.pull += 1
      return json(route, 200, { animals: [], next_cursor: null })
    }

    if (path.includes('/geo/city')) {
      log.geo += 1
      return json(route, 200, { city: '宁波', district: '海曙', country: 'CN' })
    }

    if (path.includes('/weather/week')) {
      log.weather += 1
      return json(route, 200, {
        days: [{ date: '2026-07-10', weather: '晴', temp_max: 30, temp_min: 22 }],
      })
    }

    log.other.push(`${method} ${path}`)
    return json(route, 200, { ok: true })
  })
}

/** Mock camera + geolocation + force capture success for deterministic E2E */
export async function installBrowserMocks(
  page: Page,
  opts?: { /** When false, leave onboarding fresh for AP-066 first-run E2E. Default true. */
    completeOnboarding?: boolean
  },
): Promise<void> {
  const completeOnboarding = opts?.completeOnboarding !== false
  await page.addInitScript(
    ({ completeOnboarding: done }) => {
    window.__AP_FORCE_CAPTURE_SUCCESS = true
    window.__AP_FORCE_CAMERA_READY = true
    try {
      if (done) {
        localStorage.setItem(
          'animal-poke-onboarding-v2',
          JSON.stringify({
            version: 2,
            step: 'done',
            skipped: true,
            completedAt: Date.now(),
            trainingCaptureDone: true,
            active: false,
            path: 'outdoor',
            updatedAt: Date.now(),
          }),
        )
        localStorage.setItem(
          'animal-poke-onboarding-v1',
          JSON.stringify({ step: 'done', skipped: true, completedAt: Date.now() }),
        )
      } else {
        // Only seed a fresh tutorial once per browser context — never wipe on reload
        // (AP-066 resume / persistence E2E).
        const seeded = sessionStorage.getItem('__AP_ONB_FRESH_SEEDED')
        if (!seeded) {
          localStorage.removeItem('animal-poke-onboarding-v2')
          localStorage.removeItem('animal-poke-onboarding-v1')
          sessionStorage.setItem('__AP_ONB_FRESH_SEEDED', '1')
        }
      }
    } catch {}

    class FakeTrack {
      kind = 'video'
      id = 'fake-video-track'
      label = 'Fake Cam'
      enabled = true
      muted = false
      readyState: MediaStreamTrackState = 'live'
      stop() {
        this.readyState = 'ended'
        this.enabled = false
      }
      getSettings() {
        return { width: 640, height: 480, deviceId: 'fake-cam', facingMode: 'environment' }
      }
      getConstraints() {
        return {}
      }
      getCapabilities() {
        return {}
      }
      applyConstraints() {
        return Promise.resolve()
      }
      clone() {
        return this
      }
      addEventListener() {}
      removeEventListener() {}
      dispatchEvent() {
        return true
      }
      onended = null
      onmute = null
      onunmute = null
      contentHint = ''
    }

    const fakeStream = {
      id: 'fake-stream',
      active: true,
      getTracks: () => [new FakeTrack() as unknown as MediaStreamTrack],
      getVideoTracks: () => [new FakeTrack() as unknown as MediaStreamTrack],
      getAudioTracks: () => [] as MediaStreamTrack[],
      getTrackById: () => null,
      addTrack: () => {},
      removeTrack: () => {},
      clone() {
        return this as unknown as MediaStream
      },
      addEventListener() {},
      removeEventListener() {},
      dispatchEvent() {
        return true
      },
      onaddtrack: null,
      onremovetrack: null,
    }

    Object.defineProperty(navigator, 'mediaDevices', {
      configurable: true,
      value: {
        getUserMedia: async () => fakeStream as unknown as MediaStream,
        enumerateDevices: async () => [
          {
            deviceId: 'cam',
            kind: 'videoinput',
            label: 'Fake Cam',
            groupId: 'g1',
            toJSON() {
              return this
            },
          },
        ],
      },
    })

    // Geolocation
    const geo = {
      getCurrentPosition: (success: PositionCallback) => {
        success({
          coords: {
            latitude: 29.87,
            longitude: 121.55,
            accuracy: 10,
            altitude: null,
            altitudeAccuracy: null,
            heading: null,
            speed: null,
            toJSON() {
              return this
            },
          },
          timestamp: Date.now(),
          toJSON() {
            return this
          },
        } as GeolocationPosition)
      },
      watchPosition: (success: PositionCallback) => {
        geo.getCurrentPosition(success)
        return 1
      },
      clearWatch: () => {},
    }
    Object.defineProperty(navigator, 'geolocation', {
      configurable: true,
      value: geo,
    })

    Object.defineProperty(navigator, 'onLine', {
      configurable: true,
      get: () => true,
    })
  },
    { completeOnboarding },
  )

  // Playwright WebKit does not expose the `camera` permission through
  // BrowserContext.grantPermissions. The media stream itself is fully mocked
  // above, so WebKit only needs the supported geolocation permission.
  const browserName = page.context().browser()?.browserType().name()
  const permissions = browserName === 'webkit' ? ['geolocation'] : ['camera', 'geolocation']
  await page.context().grantPermissions(permissions, {
    origin: 'http://127.0.0.1:4173',
  })
}
