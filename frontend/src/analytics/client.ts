/**
 * Privacy-friendly analytics client (AP-057).
 * - Pseudo-anonymous session id
 * - No photos / tokens / precise coords
 * - Coarse location only with location consent
 * - Sample rate + offline queue + schema version
 * - Stops on consent revoke
 */

import { authedRequest } from '../auth/deviceAuth'
import { getConsent, hasScope } from '../compliance'
import { getAnalyticsPref } from '../privacy/analyticsPrefs'
import {
  ANALYTICS_SCHEMA_VERSION,
  isFunnelEventName,
  stripForbiddenFields,
  type AnalyticsEventBase,
  type CoarseLocation,
  type FunnelEventName,
  type FunnelEventProps,
} from './schema'
import { enqueueEvent, peekQueue, dequeueEvents, clearQueue, _resetQueueForTesting } from './queue'
import { getOrCreateSessionId, clearSessionForTesting, rotateSessionId } from './session'

export type AnalyticsConsentState = 'allowed' | 'denied'

export type TrackOptions = {
  /** Force include even when sample rate would drop (tests). */
  force?: boolean
  experimentId?: string
  experimentVariant?: string
  /** Optional coarse location override; still requires consent. */
  coarseLocation?: CoarseLocation
  ts?: number
  eventId?: string
}

let sampleRate = 1
let consentOverride: AnalyticsConsentState | null = null
let transport:
  | ((events: AnalyticsEventBase[]) => Promise<void>)
  | null = null
let isFlushing = false
let onlineListenerInstalled = false

function randomEventId(): string {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) {
    return crypto.randomUUID()
  }
  return `e-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`
}

/** 0–1 sample rate; default 1 (all events). */
export function setSampleRate(rate: number): void {
  if (!Number.isFinite(rate)) return
  sampleRate = Math.min(1, Math.max(0, rate))
}

export function getSampleRate(): number {
  return sampleRate
}

/**
 * Override analytics consent for tests or product kill-switch.
 * null → derive from privacy consent (denied/pending stops events).
 */
export function setAnalyticsConsent(state: AnalyticsConsentState | null): void {
  consentOverride = state
}

export function isAnalyticsAllowed(): boolean {
  if (consentOverride != null) return consentOverride === 'allowed'
  // 用户在隐私中心显式关闭分析（AP-068）
  if (getAnalyticsPref() === 'denied') return false
  const c = getConsent()
  if (c.status === 'denied' || c.status === 'pending') return false
  if (c.revokedAt != null && (c.status !== 'granted' || c.scopes.length === 0)) return false
  // Require at least granted or offline_pending with photo scope (core product consent).
  return c.status === 'granted' || c.status === 'offline_pending'
}

export function setAnalyticsTransport(
  fn: ((events: AnalyticsEventBase[]) => Promise<void>) | null,
): void {
  transport = fn
}

function shouldSample(force?: boolean): boolean {
  if (force) return true
  if (sampleRate >= 1) return true
  if (sampleRate <= 0) return false
  return Math.random() < sampleRate
}

function resolveCoarseLocation(override?: CoarseLocation): CoarseLocation | undefined {
  if (!hasScope('location')) return undefined
  if (!override) return undefined
  // Never accept lat/lng even if caller passes them nested.
  const cleaned = stripForbiddenFields(override)
  const city = typeof cleaned.city === 'string' ? cleaned.city : undefined
  const region = typeof cleaned.region === 'string' ? cleaned.region : undefined
  const country = typeof cleaned.country === 'string' ? cleaned.country : undefined
  if (!city && !region && !country) return undefined
  return { city, region, country }
}

export function buildEvent<N extends FunnelEventName>(
  name: N,
  props: FunnelEventProps[N] | Record<string, unknown>,
  options: TrackOptions = {},
): AnalyticsEventBase {
  const session_id = getOrCreateSessionId()
  const cleanedProps = stripForbiddenFields({ ...(props as Record<string, unknown>) })
  const event: AnalyticsEventBase = {
    schema_version: ANALYTICS_SCHEMA_VERSION,
    session_id,
    name,
    ts: options.ts ?? Date.now(),
    event_id: options.eventId ?? randomEventId(),
    props: cleanedProps,
  }
  const coarse = resolveCoarseLocation(options.coarseLocation)
  if (coarse) event.coarse_location = coarse
  if (options.experimentId) event.experiment_id = options.experimentId
  if (options.experimentVariant) event.experiment_variant = options.experimentVariant
  return stripForbiddenFields(event)
}

/**
 * Track a funnel event. No-ops when consent revoked / sampling drops.
 * Never throws; never attaches photos/tokens/precise coords.
 */
export async function trackEvent<N extends FunnelEventName>(
  name: N,
  props: FunnelEventProps[N] | Record<string, unknown>,
  options: TrackOptions = {},
): Promise<AnalyticsEventBase | null> {
  try {
    if (!isFunnelEventName(name)) return null
    if (!isAnalyticsAllowed()) return null
    if (!shouldSample(options.force)) return null

    const event = buildEvent(name, props, options)

    if (typeof navigator !== 'undefined' && !navigator.onLine) {
      enqueueEvent(event)
      return event
    }

    try {
      await sendBatch([event])
    } catch {
      enqueueEvent(event)
    }
    return event
  } catch {
    return null
  }
}

async function defaultTransport(events: AnalyticsEventBase[]): Promise<void> {
  await authedRequest({
    method: 'POST',
    path: '/api/v1/analytics/events',
    body: JSON.stringify({
      schema_version: ANALYTICS_SCHEMA_VERSION,
      events,
    }),
    allowRetry: true,
    idempotencyKey: `analytics-${events[0]?.event_id ?? Date.now()}`,
  })
}

async function sendBatch(events: AnalyticsEventBase[]): Promise<void> {
  if (events.length === 0) return
  const payload = events.map((e) => stripForbiddenFields(e))
  if (transport) {
    await transport(payload)
    return
  }
  await defaultTransport(payload)
}

/** Flush offline queue to backend. */
export async function flushAnalyticsQueue(): Promise<number> {
  if (isFlushing) return 0
  if (!isAnalyticsAllowed()) {
    // Consent revoke: drop queued events rather than ship.
    clearQueue()
    return 0
  }
  const pending = peekQueue()
  if (pending.length === 0) return 0
  isFlushing = true
  let sent = 0
  try {
    // Send in chunks of 20
    const chunkSize = 20
    for (let i = 0; i < pending.length; i += chunkSize) {
      const chunk = pending.slice(i, i + chunkSize)
      try {
        await sendBatch(chunk)
        dequeueEvents(chunk.map((e) => e.event_id))
        sent += chunk.length
      } catch {
        break
      }
    }
  } finally {
    isFlushing = false
  }
  return sent
}

export function installAnalyticsOnlineListener(): void {
  if (onlineListenerInstalled) return
  if (typeof window === 'undefined') return
  onlineListenerInstalled = true
  window.addEventListener('online', () => {
    flushAnalyticsQueue().catch(() => {})
  })
}

/** Call when privacy consent is revoked — clears queue and blocks further events. */
export function onAnalyticsConsentRevoked(): void {
  setAnalyticsConsent('denied')
  clearQueue()
  rotateSessionId()
}

export function _resetAnalyticsForTesting(): void {
  sampleRate = 1
  consentOverride = null
  transport = null
  isFlushing = false
  onlineListenerInstalled = false
  _resetQueueForTesting()
  clearSessionForTesting()
}
