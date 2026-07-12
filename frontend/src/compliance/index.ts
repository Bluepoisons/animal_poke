/**
 * 合规管理模块 — 隐私授权（scope/version）、未成年人保护、数据删除。
 * 服务端权威版本：v1；scope: photo | location | precise_location
 */

import { authedRequest } from '../auth/deviceAuth'

export type ConsentStatus = 'granted' | 'denied' | 'pending' | 'offline_pending'

export type ConsentScope = 'photo' | 'location' | 'precise_location'

/** 与后端 PrivacyHandler / Vision RequireConsent 对齐 */
export const SERVER_CONSENT_VERSION = 'v1'

export const ALL_SCOPES: readonly ConsentScope[] = [
  'photo',
  'location',
  'precise_location',
] as const

export interface ConsentRecord {
  status: ConsentStatus
  grantedAt: number | null
  /** 本地展示版本；与 SERVER_CONSENT_VERSION 比较是否过期 */
  version: string
  scopes: ConsentScope[]
  /** 服务端是否已确认（离线同意为 false） */
  serverSynced: boolean
  revokedAt: number | null
  updatedAt: number
}

export interface AgeVerification {
  isMinor: boolean
  verifiedAt: number
}

export interface DataDeletionRequest {
  requestId: string
  requestedAt: number
  status: 'pending' | 'processing' | 'completed'
}

const CONSENT_KEY = 'animal-poke-consent'
const AGE_KEY = 'animal-poke-age-verification'
const PENDING_KEY = 'animal-poke-consent-pending'

/** @deprecated 使用 SERVER_CONSENT_VERSION */
export const CONSENT_VERSION = SERVER_CONSENT_VERSION

function defaultRecord(partial?: Partial<ConsentRecord>): ConsentRecord {
  return {
    status: 'pending',
    grantedAt: null,
    version: SERVER_CONSENT_VERSION,
    scopes: [],
    serverSynced: false,
    revokedAt: null,
    updatedAt: Date.now(),
    ...partial,
  }
}

function parseScopes(raw: unknown): ConsentScope[] {
  if (!Array.isArray(raw)) return []
  return raw.filter((s): s is ConsentScope =>
    typeof s === 'string' && (ALL_SCOPES as readonly string[]).includes(s),
  )
}

/** 兼容旧本地记录 { status, grantedAt, version } */
export function normalizeConsentRecord(raw: unknown): ConsentRecord {
  if (!raw || typeof raw !== 'object') return defaultRecord()
  const o = raw as Record<string, unknown>
  const status = (o.status as ConsentStatus) || 'pending'
  let scopes = parseScopes(o.scopes)
  // 旧版 granted 无 scopes → 视为 photo+location
  if (status === 'granted' && scopes.length === 0) {
    scopes = ['photo', 'location']
  }
  return defaultRecord({
    status,
    grantedAt: typeof o.grantedAt === 'number' ? o.grantedAt : null,
    version: typeof o.version === 'string' ? o.version : SERVER_CONSENT_VERSION,
    scopes,
    serverSynced: Boolean(o.serverSynced),
    revokedAt: typeof o.revokedAt === 'number' ? o.revokedAt : null,
    updatedAt: typeof o.updatedAt === 'number' ? o.updatedAt : Date.now(),
  })
}

function persist(record: ConsentRecord): ConsentRecord {
  try {
    localStorage.setItem(CONSENT_KEY, JSON.stringify(record))
  } catch {
    /* ignore */
  }
  try {
    if (typeof window !== 'undefined') {
      window.dispatchEvent(new Event('animal-poke-consent-changed'))
    }
  } catch {
    /* ignore */
  }
  return record
}

export function getConsent(): ConsentRecord {
  if (typeof localStorage === 'undefined') return defaultRecord()
  try {
    const raw = localStorage.getItem(CONSENT_KEY)
    if (!raw) return defaultRecord()
    return normalizeConsentRecord(JSON.parse(raw))
  } catch {
    return defaultRecord()
  }
}

export function hasScope(scope: ConsentScope, record = getConsent()): boolean {
  if (record.status !== 'granted' && record.status !== 'offline_pending') return false
  // offline_pending 仅本地，不算服务端已授权；UI 可提示
  if (record.status === 'offline_pending') return record.scopes.includes(scope)
  return record.serverSynced && record.scopes.includes(scope)
}

/** 发现/捕获需要 photo；定位能力需要 location */
export function canUseDiscover(record = getConsent()): boolean {
  return hasScope('photo', record) && record.serverSynced && record.status === 'granted'
}

export function canUseLocation(record = getConsent()): boolean {
  return hasScope('location', record) && record.serverSynced && record.status === 'granted'
}

export function isReadOnlyMode(record = getConsent()): boolean {
  return record.status === 'denied' || record.status === 'pending' || !canUseDiscover(record)
}

export function scopesToServerString(scopes: ConsentScope[]): string {
  return [...new Set(scopes)].join(',')
}

export function parseServerScopeString(scope: string | undefined): ConsentScope[] {
  if (!scope) return []
  return scope
    .split(',')
    .map((s) => s.trim())
    .filter((s): s is ConsentScope => (ALL_SCOPES as readonly string[]).includes(s))
}

export async function postConsentToServer(opts: {
  scopes: ConsentScope[]
  revoke?: boolean
  version?: string
}): Promise<{ ok: boolean; error?: string }> {
  const version = opts.version || SERVER_CONSENT_VERSION
  try {
    await authedRequest({
      method: 'POST',
      path: '/api/v1/privacy/consent',
      body: JSON.stringify({
        version,
        scope: scopesToServerString(opts.scopes),
        revoke: Boolean(opts.revoke),
      }),
      timeoutMs: 12_000,
      allowRetry: true,
    })
    return { ok: true }
  } catch (err) {
    const message = err instanceof Error ? err.message : 'consent_sync_failed'
    return { ok: false, error: message }
  }
}

/**
 * 授予同意：先打服务端，成功后再写本地 granted。
 * 离线/失败 → offline_pending，不伪装为 serverSynced。
 */
export async function grantConsentAsync(
  scopes: ConsentScope[] = ['photo', 'location'],
): Promise<ConsentRecord> {
  const unique = [...new Set(scopes)]
  const online = typeof navigator === 'undefined' ? true : navigator.onLine

  if (!online) {
    const record = defaultRecord({
      status: 'offline_pending',
      grantedAt: Date.now(),
      version: SERVER_CONSENT_VERSION,
      scopes: unique,
      serverSynced: false,
    })
    try {
      localStorage.setItem(PENDING_KEY, JSON.stringify({ scopes: unique, at: Date.now() }))
    } catch {
      /* ignore */
    }
    return persist(record)
  }

  const res = await postConsentToServer({ scopes: unique, revoke: false })
  if (!res.ok) {
    const record = defaultRecord({
      status: 'offline_pending',
      grantedAt: Date.now(),
      version: SERVER_CONSENT_VERSION,
      scopes: unique,
      serverSynced: false,
    })
    return persist(record)
  }

  try {
    localStorage.removeItem(PENDING_KEY)
  } catch {
    /* ignore */
  }

  return persist(
    defaultRecord({
      status: 'granted',
      grantedAt: Date.now(),
      version: SERVER_CONSENT_VERSION,
      scopes: unique,
      serverSynced: true,
      revokedAt: null,
    }),
  )
}

/** 同步 grant（测试/兼容旧 API）：仅写本地并标记 serverSynced=true（测试环境） */
export function grantConsent(
  scopes: ConsentScope[] = ['photo', 'location'],
): ConsentRecord {
  const record = defaultRecord({
    status: 'granted',
    grantedAt: Date.now(),
    version: SERVER_CONSENT_VERSION,
    scopes: [...new Set(scopes)],
    serverSynced: true,
  })
  return persist(record)
}

export async function revokeConsentAsync(
  scopes?: ConsentScope[],
): Promise<ConsentRecord> {
  const current = getConsent()
  const target = scopes && scopes.length ? scopes : current.scopes
  const online = typeof navigator === 'undefined' ? true : navigator.onLine

  if (online) {
    await postConsentToServer({
      scopes: target.length ? target : ['photo', 'location'],
      revoke: true,
    })
  }

  // 部分撤回：从 scopes 移除
  if (scopes && scopes.length && current.scopes.some((s) => !scopes.includes(s))) {
    const remaining = current.scopes.filter((s) => !scopes.includes(s))
    if (remaining.length) {
      if (online) {
        await postConsentToServer({ scopes: remaining, revoke: false })
      }
      return persist(
        defaultRecord({
          status: online ? 'granted' : 'offline_pending',
          grantedAt: current.grantedAt,
          scopes: remaining,
          serverSynced: online,
        }),
      )
    }
  }

  return persist(
    defaultRecord({
      status: 'denied',
      grantedAt: null,
      scopes: [],
      serverSynced: online,
      revokedAt: Date.now(),
    }),
  )
}

export function revokeConsent(): ConsentRecord {
  return persist(
    defaultRecord({
      status: 'denied',
      grantedAt: null,
      scopes: [],
      serverSynced: true,
      revokedAt: Date.now(),
    }),
  )
}

export function isConsentGranted(): boolean {
  const c = getConsent()
  return c.status === 'granted' && c.serverSynced && c.scopes.includes('photo')
}

/** 启动时刷离线 pending 同意 */
export async function flushPendingConsent(): Promise<ConsentRecord | null> {
  const c = getConsent()
  if (c.status !== 'offline_pending') return null
  if (typeof navigator !== 'undefined' && !navigator.onLine) return c
  return grantConsentAsync(c.scopes.length ? c.scopes : ['photo', 'location'])
}

export function verifyAge(birthYear: number): AgeVerification {
  const currentYear = new Date().getFullYear()
  const age = currentYear - birthYear
  const result: AgeVerification = {
    isMinor: age < 18,
    verifiedAt: Date.now(),
  }
  try {
    localStorage.setItem(AGE_KEY, JSON.stringify(result))
  } catch {
    /* ignore */
  }
  return result
}

export function getAgeVerification(): AgeVerification | null {
  if (typeof localStorage === 'undefined') return null
  try {
    const raw = localStorage.getItem(AGE_KEY)
    if (!raw) return null
    return JSON.parse(raw) as AgeVerification
  } catch {
    return null
  }
}

export const MINOR_DAILY_LIMIT_SECONDS = 5400
export const MINOR_ALLOWED_HOURS = { start: 8, end: 22 }

export function isMinorAllowedTime(now: Date = new Date()): boolean {
  const hour = now.getHours()
  return hour >= MINOR_ALLOWED_HOURS.start && hour < MINOR_ALLOWED_HOURS.end
}

/** Whether to apply stricter minor location/social defaults (AP-056). Default true. */
export let STRICT_MINOR_DEFAULTS = true

/** Test/config helper to toggle strict minor defaults. */
export function setStrictMinorDefaults(enabled: boolean): void {
	STRICT_MINOR_DEFAULTS = enabled
}

export type LocationScopeDefault = 'none' | 'city' | 'precise'

export interface MinorAccountDefaults {
  audience: 'minor' | 'adult'
  strict: boolean
  playHoursStart: number
  playHoursEnd: number
  dailyLimitSeconds: number
  locationScope: LocationScopeDefault
  preciseLocationDefault: boolean
  shareLocationDefault: boolean
  socialEnabled: boolean
  friendsDefault: boolean
  publicProfileDefault: boolean
  shareCaptureDefault: boolean
}

/** Adult account defaults (consent still required for location/photo). */
export function getAdultAccountDefaults(): MinorAccountDefaults {
  return {
    audience: 'adult',
    strict: false,
    playHoursStart: 0,
    playHoursEnd: 24,
    dailyLimitSeconds: 0,
    locationScope: 'city',
    preciseLocationDefault: false,
    shareLocationDefault: false,
    socialEnabled: true,
    friendsDefault: true,
    publicProfileDefault: false,
    shareCaptureDefault: true,
  }
}

/**
 * Minor account defaults. When strict (default), location and social are off
 * until guardian-approved consent expands scope.
 */
export function getMinorAccountDefaults(
  strict: boolean = STRICT_MINOR_DEFAULTS,
): MinorAccountDefaults {
  if (strict) {
    return {
      audience: 'minor',
      strict: true,
      playHoursStart: MINOR_ALLOWED_HOURS.start,
      playHoursEnd: MINOR_ALLOWED_HOURS.end,
      dailyLimitSeconds: MINOR_DAILY_LIMIT_SECONDS,
      locationScope: 'none',
      preciseLocationDefault: false,
      shareLocationDefault: false,
      socialEnabled: false,
      friendsDefault: false,
      publicProfileDefault: false,
      shareCaptureDefault: false,
    }
  }
  return {
    audience: 'minor',
    strict: false,
    playHoursStart: MINOR_ALLOWED_HOURS.start,
    playHoursEnd: MINOR_ALLOWED_HOURS.end,
    dailyLimitSeconds: MINOR_DAILY_LIMIT_SECONDS,
    locationScope: 'city',
    preciseLocationDefault: false,
    shareLocationDefault: false,
    socialEnabled: true,
    friendsDefault: false,
    publicProfileDefault: false,
    shareCaptureDefault: false,
  }
}

export function resolveAccountDefaults(isMinor: boolean): MinorAccountDefaults {
  if (isMinor) return getMinorAccountDefaults(STRICT_MINOR_DEFAULTS)
  return getAdultAccountDefaults()
}

export function createDeletionRequest(): DataDeletionRequest {
  return {
    requestId:
      typeof crypto !== 'undefined' && crypto.randomUUID
        ? crypto.randomUUID()
        : `del-${Date.now()}`,
    requestedAt: Date.now(),
    status: 'pending',
  }
}

export function isConsentOutdated(): boolean {
  const consent = getConsent()
  if (consent.status === 'pending' || consent.status === 'denied') return false
  return consent.version !== SERVER_CONSENT_VERSION
}
