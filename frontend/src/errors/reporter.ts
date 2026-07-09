import type { ErrorReport } from './types'

declare const __APP_VERSION__: string

const REPORT_ENDPOINT = '/api/v1/errors/report'
const MAX_QUEUE = 20
const RETRY_DELAYS = [1000, 3000, 10000]

const offlineQueue: ErrorReport[] = []
let isFlushing = false

/**
 * 上报错误到后端。
 * - 在线：立即发送，失败则入队
 * - 离线：入队，等网络恢复后批量发送
 * - 上报本身永不抛出异常（静默失败）
 */
export async function reportError(report: ErrorReport): Promise<void> {
  try {
    if (typeof navigator !== 'undefined' && !navigator.onLine) {
      enqueue(report)
      return
    }
    await sendWithRetry(report, 0)
  } catch {
    enqueue(report)
  }
}

async function sendWithRetry(report: ErrorReport, attempt: number): Promise<void> {
  try {
    const resp = await fetch(REPORT_ENDPOINT, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(report),
      keepalive: true,
    })
    if (!resp.ok && resp.status >= 500 && attempt < RETRY_DELAYS.length) {
      await sleep(RETRY_DELAYS[attempt])
      await sendWithRetry(report, attempt + 1)
    }
  } catch (err) {
    if (attempt < RETRY_DELAYS.length) {
      await sleep(RETRY_DELAYS[attempt])
      await sendWithRetry(report, attempt + 1)
      return
    }
    throw err
  }
}

function enqueue(report: ErrorReport): void {
  if (offlineQueue.length >= MAX_QUEUE) {
    offlineQueue.shift()
  }
  offlineQueue.push(report)
}

/** 网络恢复后批量发送队列中的错误 */
export async function flushQueue(): Promise<void> {
  if (isFlushing || offlineQueue.length === 0) return
  isFlushing = true
  try {
    while (offlineQueue.length > 0) {
      const report = offlineQueue[0]
      try {
        await sendWithRetry(report, 0)
        offlineQueue.shift()
      } catch {
        break
      }
    }
  } finally {
    isFlushing = false
  }
}

function sleep(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms))
}

let onlineListenerInstalled = false
export function installOnlineListener(): void {
  if (onlineListenerInstalled) return
  onlineListenerInstalled = true
  window.addEventListener('online', () => {
    flushQueue().catch(() => {})
  })
}

/** 仅供测试使用：重置模块状态 */
export function _resetForTesting(): void {
  offlineQueue.length = 0
  isFlushing = false
  onlineListenerInstalled = false
}
