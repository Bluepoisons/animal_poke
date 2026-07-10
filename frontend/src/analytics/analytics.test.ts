import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import {
  ANALYTICS_SCHEMA_VERSION,
  FORBIDDEN_FIELD_KEYS,
  stripForbiddenFields,
  isForbiddenKey,
  buildEvent,
  trackEvent,
  flushAnalyticsQueue,
  setSampleRate,
  setAnalyticsConsent,
  setAnalyticsTransport,
  isAnalyticsAllowed,
  onAnalyticsConsentRevoked,
  _resetAnalyticsForTesting,
  peekQueue,
  queueSize,
  getOrCreateSessionId,
  computeDetectRates,
  computeStageDropOff,
  validateExperiment,
  defineExperiment,
  assignVariant,
  type AnalyticsEventBase,
} from './index'
import { grantConsent, revokeConsent } from '../compliance'

describe('analytics schema privacy', () => {
  beforeEach(() => {
    _resetAnalyticsForTesting()
    localStorage.clear()
    grantConsent(['photo', 'location'])
    setAnalyticsConsent('allowed')
  })

  afterEach(() => {
    _resetAnalyticsForTesting()
    localStorage.clear()
  })

  it('strips photos, tokens, and precise coordinates from props', () => {
    const dirty = {
      outcome: 'success',
      photo: 'data:image/png;base64,AAAA',
      imageBase64: 'zzzz',
      token: 'secret-token',
      access_token: 'jwt-xxx',
      lat: 31.2304,
      lng: 121.4737,
      latitude: 31.23,
      longitude: 121.47,
      species_bucket: 'known',
    }
    const clean = stripForbiddenFields(dirty)
    expect(clean).toEqual({ outcome: 'success', species_bucket: 'known' })
    for (const key of ['photo', 'token', 'lat', 'lng', 'access_token']) {
      expect(Object.keys(clean)).not.toContain(key)
    }
  })

  it('buildEvent never emits forbidden keys even if nested', () => {
    const event = buildEvent(
      'detect_result',
      {
        outcome: 'success',
        image: 'blob',
        meta: { jwt: 'abc', ok: true },
      } as never,
      { force: true },
    )
    const json = JSON.stringify(event)
    for (const key of FORBIDDEN_FIELD_KEYS) {
      // allow keys that are substrings only when not present as JSON keys
      if (['position'].includes(key)) continue
      expect(json.includes(`"${key}"`)).toBe(false)
    }
    expect(event.props.meta).toEqual({ ok: true })
    expect(event.schema_version).toBe(ANALYTICS_SCHEMA_VERSION)
    expect(event.session_id.length).toBeGreaterThanOrEqual(8)
    expect(isForbiddenKey('accessToken')).toBe(true)
  })

  it('uses stable pseudo-anonymous session id (not device token)', () => {
    const a = getOrCreateSessionId()
    const b = getOrCreateSessionId()
    expect(a).toBe(b)
    expect(a).not.toMatch(/bearer|jwt/i)
    localStorage.setItem('ap_access_token', 'tok-should-not-leak')
    const event = buildEvent('auth', { result: 'success' })
    expect(JSON.stringify(event)).not.toContain('tok-should-not-leak')
  })

  it('omits coarse_location without location consent', () => {
    grantConsent(['photo']) // no location
    setAnalyticsConsent('allowed')
    const event = buildEvent(
      'scan',
      { result: 'started' },
      { coarseLocation: { city: 'Shanghai' } },
    )
    expect(event.coarse_location).toBeUndefined()
  })

  it('includes coarse city only when location consented; never lat/lng', () => {
    grantConsent(['photo', 'location'])
    setAnalyticsConsent('allowed')
    const event = buildEvent(
      'scan',
      { result: 'started' },
      {
        coarseLocation: {
          city: 'Shanghai',
          // @ts-expect-error intentional forbidden field
          lat: 31.2,
          lng: 121.4,
        },
      },
    )
    expect(event.coarse_location).toEqual({
      city: 'Shanghai',
      region: undefined,
      country: undefined,
    })
    expect(JSON.stringify(event)).not.toMatch(/31\.2|121\.4/)
  })
})

describe('analytics offline + consent', () => {
  beforeEach(() => {
    _resetAnalyticsForTesting()
    localStorage.clear()
    grantConsent(['photo', 'location'])
    setAnalyticsConsent('allowed')
    setSampleRate(1)
  })

  afterEach(() => {
    _resetAnalyticsForTesting()
    vi.unstubAllGlobals()
    localStorage.clear()
  })

  it('queues events when offline and flushes later', async () => {
    vi.stubGlobal('navigator', { onLine: false })
    const sent: AnalyticsEventBase[][] = []
    setAnalyticsTransport(async (events) => {
      sent.push(events)
    })

    const e = await trackEvent('camera_ok', { result: 'granted' }, { force: true })
    expect(e).not.toBeNull()
    expect(queueSize()).toBe(1)
    expect(sent).toHaveLength(0)

    vi.stubGlobal('navigator', { onLine: true })
    const n = await flushAnalyticsQueue()
    expect(n).toBe(1)
    expect(queueSize()).toBe(0)
    expect(sent).toHaveLength(1)
    expect(sent[0][0].name).toBe('camera_ok')
    expect(JSON.stringify(sent[0])).not.toMatch(/photo|token|latitude/i)
  })

  it('stops tracking after consent revoke and drops queue', async () => {
    vi.stubGlobal('navigator', { onLine: false })
    setAnalyticsTransport(async () => {})

    await trackEvent('scan', { result: 'started' }, { force: true })
    expect(queueSize()).toBe(1)

    revokeConsent()
    onAnalyticsConsentRevoked()
    expect(isAnalyticsAllowed()).toBe(false)
    expect(queueSize()).toBe(0)

    const blocked = await trackEvent('detect_result', { outcome: 'success' }, { force: true })
    expect(blocked).toBeNull()
    expect(peekQueue()).toHaveLength(0)
  })

  it('respects sample rate 0', async () => {
    setSampleRate(0)
    const e = await trackEvent('auth', { result: 'success' })
    expect(e).toBeNull()
  })

  it('sends online events via transport', async () => {
    vi.stubGlobal('navigator', { onLine: true })
    const sent: AnalyticsEventBase[] = []
    setAnalyticsTransport(async (events) => {
      sent.push(...events)
    })
    await trackEvent(
      'battle_end',
      { result: 'win', mode: 'pve' },
      { force: true, experimentId: 'exp1', experimentVariant: 'A' },
    )
    expect(sent).toHaveLength(1)
    expect(sent[0].experiment_id).toBe('exp1')
    expect(sent[0].experiment_variant).toBe('A')
  })
})

describe('analytics metrics', () => {
  it('computes success/unknown rates and stage drop-off', () => {
    const events: AnalyticsEventBase[] = [
      base('auth', 's1'),
      base('camera_ok', 's1'),
      base('scan', 's1'),
      base('detect_result', 's1', { outcome: 'success' }),
      base('detect_result', 's2', { outcome: 'unknown' }),
      base('detect_result', 's3', { outcome: 'error' }),
      base('auth', 's2'),
      base('camera_ok', 's2'),
    ]
    const rates = computeDetectRates(events)
    expect(rates.total).toBe(3)
    expect(rates.successRate).toBeCloseTo(1 / 3)
    expect(rates.unknownRate).toBeCloseTo(1 / 3)

    const drop = computeStageDropOff(events)
    expect(drop[0].stage).toBe('auth')
    expect(drop[0].count).toBe(2)
    expect(drop[1].stage).toBe('camera_ok')
    expect(drop[1].count).toBe(2)
  })
})

describe('analytics experiments', () => {
  it('rejects experiments missing stop condition / sample size / welfare', () => {
    const bad = validateExperiment({
      id: 'x',
      variants: ['A', 'B'],
    })
    expect(bad.ok).toBe(false)
    if (!bad.ok) {
      expect(bad.errors).toContain('missing_sample_size')
      expect(bad.errors).toContain('missing_stop_condition')
      expect(bad.errors).toContain('missing_welfare_guardrails')
    }
  })

  it('defines valid experiment and assigns variant', () => {
    const exp = defineExperiment({
      id: 'capture-cta',
      name: 'Capture CTA copy',
      variants: ['control', 'treatment'],
      sampleSize: 1000,
      stopCondition: {
        maxAssignments: 1000,
        maxDurationMs: 14 * 24 * 3600_000,
        maxHarmRate: 0.01,
      },
      welfareGuardrails: {
        noAnimalHarm: true,
        maxCaptureAttemptsPerSession: 5,
        animalSafetyNote: 'No incentives to chase or stress wildlife.',
        excludeMinors: true,
      },
    })
    const a = assignVariant(exp, 'session-aaa')
    const b = assignVariant(exp, 'session-aaa')
    expect(a.stopped).toBe(false)
    expect(a.variant).toBe(b.variant)
    expect(['control', 'treatment']).toContain(a.variant)
  })

  it('stops when sample size reached', () => {
    const exp = defineExperiment({
      id: 'stop-test',
      variants: ['A', 'B'],
      sampleSize: 10,
      stopCondition: { maxAssignments: 10 },
      welfareGuardrails: {
        noAnimalHarm: true,
        maxCaptureAttemptsPerSession: 3,
        animalSafetyNote: 'Safe.',
      },
      assignments: 10,
    })
    const r = assignVariant(exp, 's')
    expect(r.stopped).toBe(true)
    expect(r.reason).toBe('sample_size')
  })
})

function base(
  name: AnalyticsEventBase['name'],
  session_id: string,
  props: Record<string, unknown> = {},
): AnalyticsEventBase {
  return {
    schema_version: ANALYTICS_SCHEMA_VERSION,
    session_id,
    name,
    ts: Date.now(),
    event_id: `${name}-${session_id}-${Math.random()}`,
    props,
  }
}
