import type { ErrorReport } from './types'
import { reportError } from './reporter'

declare const __APP_VERSION__: string

let installed = false

export function setupGlobalErrorHandlers(): void {
  if (installed) return
  installed = true

  window.addEventListener('error', (event: ErrorEvent) => {
    const report: ErrorReport = {
      id: typeof crypto !== 'undefined' && crypto.randomUUID
        ? crypto.randomUUID()
        : `${Date.now()}-${Math.random()}`,
      type: 'window',
      name: event.error?.name ?? 'Error',
      message: event.message || 'Unknown error',
      stack: event.error?.stack,
      timestamp: Date.now(),
      page: window.location.pathname,
      appVersion: typeof __APP_VERSION__ !== 'undefined' ? __APP_VERSION__ : '0.0.0',
      userAgent: navigator.userAgent,
      context: {
        filename: event.filename,
        lineno: event.lineno,
        colno: event.colno,
      },
      online: navigator.onLine,
    }
    reportError(report).catch(() => {})
  })

  window.addEventListener('unhandledrejection', (event: PromiseRejectionEvent) => {
    const reason = event.reason
    const error = reason instanceof Error ? reason : new Error(String(reason))
    const report: ErrorReport = {
      id: typeof crypto !== 'undefined' && crypto.randomUUID
        ? crypto.randomUUID()
        : `${Date.now()}-${Math.random()}`,
      type: 'unhandledrejection',
      name: error.name,
      message: error.message || 'Unhandled promise rejection',
      stack: error.stack,
      timestamp: Date.now(),
      page: window.location.pathname,
      appVersion: typeof __APP_VERSION__ !== 'undefined' ? __APP_VERSION__ : '0.0.0',
      userAgent: navigator.userAgent,
      context: {
        reason: typeof reason === 'object' ? String(reason) : reason,
      },
      online: navigator.onLine,
    }
    reportError(report).catch(() => {})
    event.preventDefault()
  })
}

/** 仅供测试使用 */
export function _resetGlobalHandlersForTesting(): void {
  installed = false
}
