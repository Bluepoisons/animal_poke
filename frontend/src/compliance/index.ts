/**
 * 合规管理模块 — 隐私授权、未成年人保护、数据删除请求。
 */

export type ConsentStatus = 'granted' | 'denied' | 'pending'

export interface ConsentRecord {
  status: ConsentStatus
  grantedAt: number | null
  version: string
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
const CONSENT_VERSION = '1.0.0'
const AGE_KEY = 'animal-poke-age-verification'

export function getConsent(): ConsentRecord {
  if (typeof localStorage === 'undefined') {
    return { status: 'pending', grantedAt: null, version: CONSENT_VERSION }
  }
  try {
    const raw = localStorage.getItem(CONSENT_KEY)
    if (!raw) return { status: 'pending', grantedAt: null, version: CONSENT_VERSION }
    return JSON.parse(raw) as ConsentRecord
  } catch {
    return { status: 'pending', grantedAt: null, version: CONSENT_VERSION }
  }
}

export function grantConsent(): ConsentRecord {
  const record: ConsentRecord = {
    status: 'granted',
    grantedAt: Date.now(),
    version: CONSENT_VERSION,
  }
  try {
    localStorage.setItem(CONSENT_KEY, JSON.stringify(record))
  } catch { /* ignore */ }
  return record
}

export function revokeConsent(): ConsentRecord {
  const record: ConsentRecord = {
    status: 'denied',
    grantedAt: null,
    version: CONSENT_VERSION,
  }
  try {
    localStorage.setItem(CONSENT_KEY, JSON.stringify(record))
  } catch { /* ignore */ }
  return record
}

export function isConsentGranted(): boolean {
  return getConsent().status === 'granted'
}

/** 未成年人判定（18 岁以下为未成年人） */
export function verifyAge(birthYear: number): AgeVerification {
  const currentYear = new Date().getFullYear()
  const age = currentYear - birthYear
  const result: AgeVerification = {
    isMinor: age < 18,
    verifiedAt: Date.now(),
  }
  try {
    localStorage.setItem(AGE_KEY, JSON.stringify(result))
  } catch { /* ignore */ }
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

/** 未成年人限制：每日游戏时长上限（秒） */
export const MINOR_DAILY_LIMIT_SECONDS = 5400 // 90 分钟

/** 未成年人限制：每日可用时段 */
export const MINOR_ALLOWED_HOURS = { start: 8, end: 22 }

/** 检查未成年人当前是否在允许游戏时段内 */
export function isMinorAllowedTime(now: Date = new Date()): boolean {
  const hour = now.getHours()
  return hour >= MINOR_ALLOWED_HOURS.start && hour < MINOR_ALLOWED_HOURS.end
}

/** 创建数据删除请求 */
export function createDeletionRequest(): DataDeletionRequest {
  const request: DataDeletionRequest = {
    requestId: typeof crypto !== 'undefined' && crypto.randomUUID
      ? crypto.randomUUID()
      : `del-${Date.now()}`,
    requestedAt: Date.now(),
    status: 'pending',
  }
  return request
}

/** 检查授权版本是否过期 */
export function isConsentOutdated(): boolean {
  const consent = getConsent()
  return consent.version !== CONSENT_VERSION
}
