export type ErrorType =
  | 'react'
  | 'window'
  | 'unhandledrejection'
  | 'indexeddb'
  | 'camera'
  | 'network'

export interface ErrorReport {
  id: string
  type: ErrorType
  name: string
  message: string
  stack?: string
  timestamp: number
  page?: string
  deviceId?: string
  appVersion: string
  /** Build/release identifier used by backend error telemetry. */
  release?: string
  userAgent: string
  context?: Record<string, unknown>
  online: boolean
}
