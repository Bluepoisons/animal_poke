import type { ErrorReport, ErrorReportPayload } from './types'
import { authedRequest } from '../auth/deviceAuth'

declare const __APP_VERSION__: string
declare const __RELEASE_SHA__: string

const MAX_QUEUE = 20
const QUEUE_KEY = 'ap_error_queue'
/** Per-fingerprint dedup window (ms). */
const DEDUP_WINDOW_MS = 60_000
/** Max distinct reports accepted per fingerprint inside the window. */
const DEDUP_MAX_PER_WINDOW = 1
/**
 * Sample rate for non-critical noise (0–1). Hard errors always pass.
 * Override via window.__AP_ERROR_SAMPLE_RATE__ in tests.
 */
const DEFAULT_SAMPLE_RATE = 1

type QueuedPayload = ErrorReportPayload & { _ts?: number }

function loadQueue(): QueuedPayload[] {
  try {
    const raw = localStorage.getItem(QUEUE_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? parsed.slice(0, MAX_QUEUE) : []
  } catch {
    return []
  }
}

function saveQueue(q: QueuedPayload[]): void {
  try {
    localStorage.setItem(QUEUE_KEY, JSON.stringify(q.slice(-MAX_QUEUE)))
  } catch {
    /* ignore quota */
  }
}

let offlineQueue: QueuedPayload[] = loadQueue()
let isFlushing = false

/** Recent fingerprints for client-side dedup (session memory). */
const recentFingerprints = new Map<string, { count: number; firstAt: number }>()

function releaseId(report?: ErrorReport): string {
  if (report?.release && report.release.trim()) return report.release.trim().slice(0, 64)
  try {
    const sha = typeof __RELEASE_SHA__ !== 'undefined' ? String(__RELEASE_SHA__ || '') : ''
    if (sha && sha !== 'dev') return sha.slice(0, 64)
  } catch {
    /* ignore */
  }
  if (report?.appVersion) return String(report.appVersion).slice(0, 64)
  return 'unknown'
}

/** Redact tokens, coords, photos, and other PII-ish fragments from free text. */
export function redactText(s?: string, maxLen = 2000): string | undefined {
  if (s == null) return s
  let out = String(s)

  // Tokens / secrets
  out = out.replace(/bearer\s+[a-z0-9._\-+=/]+/gi, 'Bearer [redacted]')
  out = out.replace(
    /(authorization|api[_-]?key|apikey|access_token|refresh_token|password|jwt|installation_secret)\s*[:=]\s*['"]?[^'"\s,;]+/gi,
    '$1=[redacted]',
  )
  out = out.replace(/\beyJ[a-zA-Z0-9_-]{10,}\.[a-zA-Z0-9_-]{10,}\.[a-zA-Z0-9_-]{10,}\b/g, '[redacted-jwt]')

  // Data URLs / base64 photo payloads
  out = out.replace(/data:image\/[a-z0-9+.-]+;base64,[a-z0-9+/=\s]+/gi, '[redacted-photo]')
  out = out.replace(/\b(photo|image|blob|thumbnail)\s*[:=]\s*['"]?[a-z0-9+/=]{40,}['"]?/gi, '$1=[redacted-photo]')

  // Precise coordinates (decimal degrees with enough precision)
  out = out.replace(
    /\b(-?\d{1,2}\.\d{4,}|-?\d{1,3}\.\d{4,})\s*[,/]\s*(-?\d{1,2}\.\d{4,}|-?\d{1,3}\.\d{4,})\b/g,
    '[redacted-coords]',
  )
  out = out.replace(
    /\b(lat(?:itude)?|lng|lon(?:gitude)?|coords?)\s*[:=]\s*-?\d+(\.\d+)?/gi,
    '$1=[redacted]',
  )

  if (/bearer\s|api[_-]?key|password|authorization|installation_secret/i.test(out) &&
      /\[redacted/i.test(out) === false) {
    // Fallback: whole string looked secret-ish without clear token shape
    if (/bearer |api[_-]?key|password|authorization/i.test(out)) {
      return '[redacted]'
    }
  }

  if (out.length > maxLen) out = out.slice(0, maxLen) + '…'
  return out
}

function isSensitiveKey(key: string): boolean {
  return /token|password|secret|authorization|api[_-]?key|photo|image|blob|lat|lng|lon|coord|gps|base64/i.test(
    key,
  )
}

/** Flatten context to string-only extra; drop sensitive keys; redact values. */
export function sanitizeExtra(
  context?: Record<string, unknown>,
  maxEntries = 12,
): Record<string, string> | undefined {
  if (!context) return undefined
  const extra: Record<string, string> = {}
  let n = 0
  for (const [k, v] of Object.entries(context)) {
    if (n >= maxEntries) break
    if (isSensitiveKey(k)) continue
    if (v == null) continue
    let str: string
    if (typeof v === 'string') str = v
    else if (typeof v === 'number' || typeof v === 'boolean') str = String(v)
    else {
      try {
        str = JSON.stringify(v)
      } catch {
        continue
      }
    }
    const redacted = redactText(str, 400)
    if (redacted == null || redacted === '') continue
    // Cap key length
    const key = k.slice(0, 40)
    extra[key] = redacted
    n++
  }
  return Object.keys(extra).length ? extra : undefined
}

/** Map client ErrorReport → backend-aligned wire payload (strict schema). */
export function toWirePayload(report: ErrorReport): ErrorReportPayload {
  const message = redactText(report.message, 500) || 'unknown'
  const stack = redactText(report.stack, 2000)
  const component = redactText(report.component || report.page, 80)
  const route =
    redactText(
      report.page ||
        (typeof window !== 'undefined' ? window.location?.pathname : undefined),
      120,
    ) || undefined
  const requestId =
    report.requestId ||
    report.id ||
    (typeof crypto !== 'undefined' && 'randomUUID' in crypto
      ? crypto.randomUUID()
      : `err-${Date.now()}`)

  const extra = sanitizeExtra({
    type: report.type,
    name: report.name,
    ...(report.context || {}),
  })

  return {
    message,
    stack,
    component: component || undefined,
    route,
    release: releaseId(report).slice(0, 64),
    level: (report.level || report.type || 'error').slice(0, 16),
    request_id: String(requestId).slice(0, 64),
    extra,
  }
}

function fingerprint(payload: ErrorReportPayload): string {
  return [payload.message, payload.component || '', payload.route || '', payload.level || '']
    .join('|')
    .slice(0, 200)
}

/** Returns true if this report should be dropped as a duplicate. */
export function shouldDedup(payload: ErrorReportPayload, now = Date.now()): boolean {
  const fp = fingerprint(payload)
  const entry = recentFingerprints.get(fp)
  if (!entry) {
    recentFingerprints.set(fp, { count: 1, firstAt: now })
    // Prune map if large
    if (recentFingerprints.size > 200) {
      for (const [k, v] of recentFingerprints) {
        if (now - v.firstAt > DEDUP_WINDOW_MS) recentFingerprints.delete(k)
      }
    }
    return false
  }
  if (now - entry.firstAt > DEDUP_WINDOW_MS) {
    recentFingerprints.set(fp, { count: 1, firstAt: now })
    return false
  }
  entry.count += 1
  return entry.count > DEDUP_MAX_PER_WINDOW
}

function sampleRate(): number {
  try {
    const w = typeof window !== 'undefined' ? (window as unknown as { __AP_ERROR_SAMPLE_RATE__?: number }) : undefined
    if (w && typeof w.__AP_ERROR_SAMPLE_RATE__ === 'number') {
      return Math.min(1, Math.max(0, w.__AP_ERROR_SAMPLE_RATE__))
    }
  } catch {
    /* ignore */
  }
  return DEFAULT_SAMPLE_RATE
}

/** Light sampling: always keep first-of-fingerprint; sample the rest. */
export function shouldSample(payload: ErrorReportPayload): boolean {
  const rate = sampleRate()
  if (rate >= 1) return true
  if (rate <= 0) return false
  // Deterministic-ish sample from fingerprint hash
  const fp = fingerprint(payload)
  let h = 0
  for (let i = 0; i < fp.length; i++) h = (h * 31 + fp.charCodeAt(i)) >>> 0
  return (h % 1000) / 1000 < rate
}

/**
 * 上报错误到后端（鉴权 + 脱敏 + 对齐 wire schema）。
 * - 在线：立即发送
 * - 离线：持久化队列，恢复后 flush
 * - 去重 / 采样
 * - 上报本身永不抛出
 */
export async function reportError(report: ErrorReport): Promise<void> {
  try {
    const payload = toWirePayload(report)
    if (shouldDedup(payload)) return
    if (!shouldSample(payload)) return

    if (typeof navigator !== 'undefined' && !navigator.onLine) {
      enqueue(payload)
      return
    }
    await send(payload)
  } catch {
    try {
      enqueue(toWirePayload(report))
    } catch {
      /* ignore */
    }
  }
}

async function send(payload: ErrorReportPayload): Promise<void> {
  // Only wire fields — never spread client ErrorReport (unknown fields rejected by backend).
  const body: ErrorReportPayload = {
    message: payload.message,
    stack: payload.stack,
    component: payload.component,
    route: payload.route,
    release: payload.release,
    level: payload.level,
    request_id: payload.request_id,
    extra: payload.extra,
  }
  // Drop undefined keys for cleaner JSON
  const clean = Object.fromEntries(
    Object.entries(body).filter(([, v]) => v !== undefined),
  ) as ErrorReportPayload

  await authedRequest({
    method: 'POST',
    path: '/api/v1/errors/report',
    body: JSON.stringify(clean),
    allowRetry: true,
    idempotencyKey: `err-${payload.request_id || fingerprint(payload).slice(0, 40)}`,
    headers: payload.request_id ? { 'X-Request-ID': payload.request_id } : undefined,
  })
}

function enqueue(payload: ErrorReportPayload): void {
  offlineQueue.push({ ...payload, _ts: Date.now() })
  if (offlineQueue.length > MAX_QUEUE) offlineQueue = offlineQueue.slice(-MAX_QUEUE)
  saveQueue(offlineQueue)
}

export async function flushQueue(): Promise<void> {
  if (isFlushing || offlineQueue.length === 0) return
  isFlushing = true
  try {
    while (offlineQueue.length > 0) {
      const report = offlineQueue[0]
      try {
        const { _ts, ...wire } = report
        void _ts
        await send(wire)
        offlineQueue.shift()
        saveQueue(offlineQueue)
      } catch {
        break
      }
    }
  } finally {
    isFlushing = false
  }
}

let onlineListenerInstalled = false
export function installOnlineListener(): void {
  if (onlineListenerInstalled) return
  onlineListenerInstalled = true
  window.addEventListener('online', () => {
    flushQueue().catch(() => {})
  })
}

export function _resetForTesting(): void {
  offlineQueue = []
  isFlushing = false
  onlineListenerInstalled = false
  recentFingerprints.clear()
  try {
    localStorage.removeItem(QUEUE_KEY)
  } catch {
    /* ignore */
  }
  try {
    if (typeof window !== 'undefined') {
      delete (window as unknown as { __AP_ERROR_SAMPLE_RATE__?: number }).__AP_ERROR_SAMPLE_RATE__
    }
  } catch {
    /* ignore */
  }
}
