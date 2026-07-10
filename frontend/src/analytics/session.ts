/**
 * Pseudo-anonymous analytics session id.
 * Random UUID — not device id, not auth token, not PII.
 */

import { safeGetItem, safeSetItem, safeRemoveItem } from '../utils/safeStorage'

const SESSION_KEY = 'ap_analytics_session_id'
const SESSION_STARTED_KEY = 'ap_analytics_session_started'

function randomId(): string {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) {
    return crypto.randomUUID()
  }
  return `s-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 12)}`
}

let memorySessionId: string | null = null
let memoryStartedAt: number | null = null

export function getOrCreateSessionId(): string {
  if (memorySessionId) return memorySessionId
  const existing = safeGetItem<string | null>(SESSION_KEY, null)
  if (typeof existing === 'string' && existing.length >= 8) {
    memorySessionId = existing
    return existing
  }
  const id = randomId()
  memorySessionId = id
  memoryStartedAt = Date.now()
  safeSetItem(SESSION_KEY, id)
  safeSetItem(SESSION_STARTED_KEY, memoryStartedAt)
  return id
}

export function getSessionStartedAt(): number {
  if (memoryStartedAt != null) return memoryStartedAt
  const stored = safeGetItem<number | null>(SESSION_STARTED_KEY, null)
  if (typeof stored === 'number') {
    memoryStartedAt = stored
    return stored
  }
  const now = Date.now()
  memoryStartedAt = now
  safeSetItem(SESSION_STARTED_KEY, now)
  return now
}

/** Rotate session (e.g. after long idle or explicit privacy reset). */
export function rotateSessionId(): string {
  memorySessionId = null
  memoryStartedAt = null
  safeRemoveItem(SESSION_KEY)
  safeRemoveItem(SESSION_STARTED_KEY)
  return getOrCreateSessionId()
}

export function clearSessionForTesting(): void {
  memorySessionId = null
  memoryStartedAt = null
  safeRemoveItem(SESSION_KEY)
  safeRemoveItem(SESSION_STARTED_KEY)
}
