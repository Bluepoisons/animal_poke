/**
 * Privacy-friendly analytics event schema (AP-057).
 * Schema registry version gates client/server compatibility.
 */

/** Bump when event payload contracts change incompatibly. */
export const ANALYTICS_SCHEMA_VERSION = 1 as const

/** Core funnel events — auth → camera → scan → detect → capture → generate → collection / trade / battle. */
export const FUNNEL_EVENT_NAMES = [
  'auth',
  'camera_ok',
  'scan',
  'detect_result',
  'capture_attempt',
  'generate_stage',
  'collection_complete',
  'trade',
  'battle_end',
] as const

export type FunnelEventName = (typeof FUNNEL_EVENT_NAMES)[number]

/** Ordered funnel stages for drop-off analysis (subset of events). */
export const FUNNEL_STAGES: readonly FunnelEventName[] = [
  'auth',
  'camera_ok',
  'scan',
  'detect_result',
  'capture_attempt',
  'generate_stage',
  'collection_complete',
] as const

/** Keys that must never appear in analytics payloads. */
export const FORBIDDEN_FIELD_KEYS = [
  'photo',
  'photos',
  'image',
  'images',
  'imageBase64',
  'image_base64',
  'imageData',
  'image_data',
  'thumbnail',
  'blob',
  'file',
  'token',
  'access_token',
  'accessToken',
  'refresh_token',
  'refreshToken',
  'jwt',
  'authorization',
  'authHeader',
  'password',
  'secret',
  'api_key',
  'apiKey',
  'bearer',
  'lat',
  'lng',
  'latitude',
  'longitude',
  'coords',
  'coordinates',
  'gps',
  'geolocation',
  'exact_location',
  'precise_location',
  'position',
] as const

export type ForbiddenFieldKey = (typeof FORBIDDEN_FIELD_KEYS)[number]

/** Coarse location only (city / region), never precise lat/lng. */
export type CoarseLocation = {
  city?: string
  region?: string
  country?: string
}

export type DetectOutcome = 'success' | 'unknown' | 'error'

export type GenerateStageName =
  | 'queued'
  | 'vision'
  | 'value'
  | 'persist'
  | 'done'
  | 'failed'

/** Allowed props per event — extra keys are stripped; forbidden keys always dropped. */
export type FunnelEventProps = {
  auth: {
    result: 'success' | 'failure' | 'refresh'
    method?: 'device'
  }
  camera_ok: {
    result: 'granted' | 'denied' | 'unavailable'
  }
  scan: {
    result: 'started' | 'frame' | 'stopped'
    fps_bucket?: 'low' | 'mid' | 'high'
  }
  detect_result: {
    outcome: DetectOutcome
    species_bucket?: 'known' | 'unknown'
    latency_ms_bucket?: 'lt500' | 'lt1500' | 'lt3000' | 'gte3000'
  }
  capture_attempt: {
    result: 'success' | 'fail' | 'cancel'
    attempt_index?: number
  }
  generate_stage: {
    stage: GenerateStageName
    result?: 'ok' | 'error' | 'skip'
  }
  collection_complete: {
    result: 'saved' | 'duplicate' | 'error'
  }
  trade: {
    result: 'offer' | 'accept' | 'reject' | 'cancel' | 'complete'
  }
  battle_end: {
    result: 'win' | 'lose' | 'draw' | 'forfeit'
    mode?: 'pve' | 'pvp'
  }
}

export type AnalyticsEventBase = {
  /** Schema registry version */
  schema_version: typeof ANALYTICS_SCHEMA_VERSION | number
  /** Pseudo-anonymous session id (not device id / not auth token) */
  session_id: string
  name: FunnelEventName
  /** Client event time (ms) */
  ts: number
  /** Event id for dedupe */
  event_id: string
  /** Coarse location only when location consent granted */
  coarse_location?: CoarseLocation
  /** Experiment assignment if any */
  experiment_id?: string
  experiment_variant?: string
  props: Record<string, unknown>
}

export function isFunnelEventName(name: string): name is FunnelEventName {
  return (FUNNEL_EVENT_NAMES as readonly string[]).includes(name)
}

export function isForbiddenKey(key: string): boolean {
  const lower = key.toLowerCase()
  return (FORBIDDEN_FIELD_KEYS as readonly string[]).some(
    (f) => f.toLowerCase() === lower || lower.includes(f.toLowerCase()),
  )
}

/** Deep-strip forbidden keys and drop nested photo/token blobs. */
export function stripForbiddenFields<T>(value: T, depth = 0): T {
  if (depth > 6 || value == null) return value
  if (Array.isArray(value)) {
    return value.map((v) => stripForbiddenFields(v, depth + 1)) as T
  }
  if (typeof value !== 'object') return value
  const out: Record<string, unknown> = {}
  for (const [k, v] of Object.entries(value as Record<string, unknown>)) {
    if (isForbiddenKey(k)) continue
    if (typeof v === 'string' && looksSensitiveString(v)) continue
    out[k] = stripForbiddenFields(v, depth + 1)
  }
  return out as T
}

function looksSensitiveString(s: string): boolean {
  if (s.length > 4000) return true // likely image base64
  if (/^data:image\//i.test(s)) return true
  if (/bearer\s+[a-z0-9._\-]+/i.test(s)) return true
  if (/eyJ[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+\./.test(s)) return true // JWT-ish
  return false
}

export function assertSchemaVersion(version: number): boolean {
  return version === ANALYTICS_SCHEMA_VERSION
}
