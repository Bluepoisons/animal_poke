export type ErrorType =
  | 'react'
  | 'window'
  | 'unhandledrejection'
  | 'indexeddb'
  | 'camera'
  | 'network'

/**
 * Wire payload for POST /api/v1/errors/report.
 * Must match backend errorReportRequest (BindStrictJSON / DisallowUnknownFields).
 * Only these fields are sent on the wire.
 */
export interface ErrorReportPayload {
  message: string
  stack?: string
  component?: string
  route?: string
  /** Build/release identifier — prefer git commit SHA in production. */
  release?: string
  level?: string
  /** Client correlation id (also sent as X-Request-ID header when available). */
  request_id?: string
  /** Non-sensitive string context only (values redacted client-side). */
  extra?: Record<string, string>
}

/**
 * Client-side error report (richer than wire payload).
 * Mapped via toWirePayload() before network send.
 */
export interface ErrorReport {
  id: string
  type: ErrorType
  name: string
  message: string
  stack?: string
  timestamp: number
  /** Route / screen name (maps to wire `route`). */
  page?: string
  /** Component / boundary name (maps to wire `component`). */
  component?: string
  deviceId?: string
  appVersion: string
  /** Build/release identifier used by backend error telemetry. */
  release?: string
  userAgent: string
  /** Arbitrary context; only safe string keys/values reach the wire. */
  context?: Record<string, unknown>
  /** Optional correlation with a prior API call. */
  requestId?: string
  online: boolean
  level?: string
}
