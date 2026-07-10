// 前端 API 客户端（基于 OpenAPI 生成类型）
// 重新生成:
//   npx openapi-typescript ../docs/openapi.yaml -o src/api/generated/schema.d.ts

export type { paths, components, operations } from './generated/schema'

import { getApiBaseUrl as getConfigBase } from '../config/publicConfig'
import { fetchWithRetry, FetchTimeoutError } from '../services/fetchWithRetry'

/** 公开配置：仅 API Base URL，禁止第三方 Key */
export function getApiBaseUrl(): string {
  return getConfigBase()
}

export type ApiErrorBody = {
  error?: string
  reason_code?: string
  request_id?: string
  retryable?: boolean
  details?: Record<string, unknown>
}

export class ApiError extends Error {
  status: number
  reasonCode?: string
  requestId?: string
  retryAfter?: number
  retryable?: boolean
  details?: Record<string, unknown>

  constructor(status: number, body: ApiErrorBody, retryAfter?: number) {
    super(body.error || `HTTP ${status}`)
    this.name = 'ApiError'
    this.status = status
    this.reasonCode = body.reason_code
    this.requestId = body.request_id
    this.retryAfter = retryAfter
    this.retryable = body.retryable
    this.details = body.details
  }
}

export type RequestOptions = {
  method?: string
  path: string
  token?: string | null
  body?: BodyInit | null
  headers?: Record<string, string>
  signal?: AbortSignal
  idempotencyKey?: string
  /** 仅对幂等请求或显式允许时重试 */
  allowRetry?: boolean
  timeoutMs?: number
}

function isIdempotent(method: string): boolean {
  return ['GET', 'HEAD', 'OPTIONS'].includes(method.toUpperCase())
}

/** 统一 API 请求：附加 Request-ID、Bearer、错误模型 */
export async function apiRequest<T = unknown>(opts: RequestOptions): Promise<T> {
  const base = getApiBaseUrl()
  const url = `${base}${opts.path.startsWith('/') ? opts.path : `/${opts.path}`}`
  const method = (opts.method || 'GET').toUpperCase()
  const requestId =
    typeof crypto !== 'undefined' && 'randomUUID' in crypto
      ? crypto.randomUUID()
      : `req-${Date.now()}`
  const headers: Record<string, string> = {
    Accept: 'application/json',
    'X-Request-ID': requestId,
    ...opts.headers,
  }
  if (opts.token) {
    headers.Authorization = `Bearer ${opts.token}`
  }
  if (opts.idempotencyKey) {
    headers['Idempotency-Key'] = opts.idempotencyKey
  }
  if (opts.body && !(opts.body instanceof FormData) && !headers['Content-Type']) {
    headers['Content-Type'] = 'application/json'
  }

  const allowRetry = opts.allowRetry ?? (isIdempotent(method) || Boolean(opts.idempotencyKey))
  const res = await fetchWithRetry(
    url,
    {
      method,
      headers,
      body: opts.body,
    },
    {
      retries: allowRetry ? 2 : 0,
      retryDelay: 400,
      timeout: opts.timeoutMs ?? 15000,
      signal: opts.signal,
      method,
      allowRetryOnWrite: allowRetry && !isIdempotent(method),
    },
  )

  const retryAfterHeader = res.headers.get('Retry-After')
  const retryAfter = retryAfterHeader ? Number(retryAfterHeader) : undefined
  const rid = res.headers.get('X-Request-ID') || requestId

  if (!res.ok) {
    let body: ApiErrorBody = { error: res.statusText, request_id: rid }
    try {
      body = { ...body, ...(await res.json()) }
    } catch {
      /* ignore */
    }
    throw new ApiError(res.status, body, Number.isFinite(retryAfter) ? retryAfter : undefined)
  }

  if (res.status === 204) {
    return undefined as T
  }
  const ct = res.headers.get('Content-Type') || ''
  if (ct.includes('application/json')) {
    return (await res.json()) as T
  }
  return (await res.text()) as T
}

export { FetchTimeoutError }
