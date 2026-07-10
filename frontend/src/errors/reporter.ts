import type { ErrorReport } from './types'
import { authedRequest } from '../auth/deviceAuth'

declare const __APP_VERSION__: string

const MAX_QUEUE = 20
const QUEUE_KEY = 'ap_error_queue'

function loadQueue(): ErrorReport[] {
  try {
    const raw = localStorage.getItem(QUEUE_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? parsed.slice(0, MAX_QUEUE) : []
  } catch {
    return []
  }
}

function saveQueue(q: ErrorReport[]): void {
  try {
    localStorage.setItem(QUEUE_KEY, JSON.stringify(q.slice(-MAX_QUEUE)))
  } catch {
    /* ignore quota */
  }
}

let offlineQueue: ErrorReport[] = loadQueue()
let isFlushing = false

function withRelease(report: ErrorReport): ErrorReport {
  const release =
    typeof __APP_VERSION__ !== 'undefined' ? __APP_VERSION__ : report.release || 'dev'
  return { ...report, release }
}

function sanitize(report: ErrorReport): ErrorReport {
  const r = withRelease(report)
  const strip = (s?: string) => {
    if (!s) return s
    if (/bearer|jwt|api[_-]?key|password|authorization/i.test(s)) return '[redacted]'
    return s.slice(0, 2000)
  }
  return {
    ...r,
    message: strip(r.message) || 'unknown',
    stack: strip(r.stack),
  }
}

/**
 * 上报错误到后端（鉴权 + 脱敏）。
 * - 在线：立即发送
 * - 离线：持久化队列，恢复后 flush
 * - 上报本身永不抛出
 */
export async function reportError(report: ErrorReport): Promise<void> {
  try {
    const payload = sanitize(report)
    if (typeof navigator !== 'undefined' && !navigator.onLine) {
      enqueue(payload)
      return
    }
    await send(payload)
  } catch {
    enqueue(sanitize(report))
  }
}

async function send(report: ErrorReport): Promise<void> {
  await authedRequest({
    method: 'POST',
    path: '/api/v1/errors/report',
    body: JSON.stringify(report),
    allowRetry: true,
    idempotencyKey: `err-${report.message?.slice(0, 40)}-${report.timestamp || Date.now()}`,
  })
}

function enqueue(report: ErrorReport): void {
  offlineQueue.push(report)
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
        await send(report)
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
  try {
    localStorage.removeItem(QUEUE_KEY)
  } catch {
    /* ignore */
  }
}
