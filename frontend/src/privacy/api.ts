/**
 * 隐私生命周期 API 客户端（导出 / 删除 / 状态轮询）— AP-068
 */

import { authedRequest } from '../auth/deviceAuth'

export type PrivacyRequestType = 'export' | 'delete'
export type PrivacyRequestStatus =
  | 'pending'
  | 'processing'
  | 'completed'
  | 'cancelled'
  | 'failed'

export type DeleteScope = 'device' | 'account'
export type ExportScope = 'device' | 'account'

export interface PrivacyRequestResult {
  request_id: string
  status: PrivacyRequestStatus | string
  scope?: string
  data?: unknown
  error_msg?: string
}

export interface PrivacyRequestRecord {
  request_id: string
  device_id?: string
  type?: PrivacyRequestType | string
  status: PrivacyRequestStatus | string
  payload?: string
  error_msg?: string
  requested_at?: string
  completed_at?: string | null
}

export interface RequestExportOptions {
  scope?: ExportScope
  cursor?: string
  signal?: AbortSignal
}

export interface RequestDeleteOptions {
  scope?: DeleteScope
  /** 账号注销必须 "DELETE" */
  confirm?: string
  reauthPassword?: string
  signal?: AbortSignal
}

export async function requestPrivacyExport(
  opts: RequestExportOptions = {},
): Promise<PrivacyRequestResult> {
  const qs = opts.cursor ? `?cursor=${encodeURIComponent(opts.cursor)}` : ''
  const body =
    opts.scope != null
      ? JSON.stringify({ scope: opts.scope })
      : undefined
  return authedRequest<PrivacyRequestResult>({
    method: 'POST',
    path: `/api/v1/privacy/export${qs}`,
    body,
    timeoutMs: 60_000,
    allowRetry: false,
    signal: opts.signal,
  })
}

export async function requestPrivacyDelete(
  opts: RequestDeleteOptions = {},
): Promise<PrivacyRequestResult> {
  const scope = opts.scope ?? 'device'
  const payload: Record<string, string> = { scope }
  if (scope === 'account') {
    payload.confirm = opts.confirm ?? 'DELETE'
    if (opts.reauthPassword) payload.reauth_password = opts.reauthPassword
  }
  return authedRequest<PrivacyRequestResult>({
    method: 'POST',
    path: '/api/v1/privacy/delete',
    body: JSON.stringify(payload),
    timeoutMs: 60_000,
    allowRetry: false,
    signal: opts.signal,
  })
}

export async function getPrivacyRequestStatus(
  requestId: string,
  signal?: AbortSignal,
): Promise<PrivacyRequestRecord> {
  return authedRequest<PrivacyRequestRecord>({
    method: 'GET',
    path: `/api/v1/privacy/requests/${encodeURIComponent(requestId)}`,
    timeoutMs: 15_000,
    allowRetry: true,
    signal,
  })
}

export function isTerminalPrivacyStatus(status: string | undefined): boolean {
  const s = (status || '').toLowerCase()
  return s === 'completed' || s === 'failed' || s === 'cancelled'
}

export function isSuccessPrivacyStatus(status: string | undefined): boolean {
  return (status || '').toLowerCase() === 'completed'
}

export interface PollOptions {
  intervalMs?: number
  timeoutMs?: number
  signal?: AbortSignal
  sleep?: (ms: number, signal?: AbortSignal) => Promise<void>
}

async function defaultSleep(ms: number, signal?: AbortSignal): Promise<void> {
  if (signal?.aborted) throw new DOMException('Aborted', 'AbortError')
  await new Promise<void>((resolve, reject) => {
    const t = setTimeout(() => {
      signal?.removeEventListener('abort', onAbort)
      resolve()
    }, ms)
    const onAbort = () => {
      clearTimeout(t)
      reject(new DOMException('Aborted', 'AbortError'))
    }
    signal?.addEventListener('abort', onAbort, { once: true })
  })
}

/**
 * 轮询 /privacy/requests/:id 直到终态或超时。
 * 超时 / 失败不伪装 completed。
 */
export async function pollPrivacyRequest(
  requestId: string,
  opts: PollOptions = {},
): Promise<PrivacyRequestRecord> {
  const intervalMs = opts.intervalMs ?? 800
  const timeoutMs = opts.timeoutMs ?? 30_000
  const sleep = opts.sleep ?? defaultSleep
  const started = Date.now()
  let last: PrivacyRequestRecord | null = null

  while (Date.now() - started < timeoutMs) {
    if (opts.signal?.aborted) throw new DOMException('Aborted', 'AbortError')
    last = await getPrivacyRequestStatus(requestId, opts.signal)
    if (isTerminalPrivacyStatus(last.status)) return last
    await sleep(intervalMs, opts.signal)
  }

  if (last && isTerminalPrivacyStatus(last.status)) return last
  throw new Error('privacy_request_timeout')
}

/**
 * 发起导出并确保拿到终态 + 数据。
 * 同步 completed 且带 data → 直接返回；processing → 轮询；payload 字符串解析。
 */
export async function exportWithStatus(
  opts: RequestExportOptions & PollOptions = {},
): Promise<{ requestId: string; status: string; data: unknown }> {
  const res = await requestPrivacyExport(opts)
  if (!res?.request_id) {
    throw new Error('export_missing_request_id')
  }

  if (isSuccessPrivacyStatus(res.status) && res.data != null) {
    return { requestId: res.request_id, status: String(res.status), data: res.data }
  }

  if (isTerminalPrivacyStatus(res.status) && !isSuccessPrivacyStatus(res.status)) {
    throw new Error(res.error_msg || `export_${res.status || 'failed'}`)
  }

  const final = await pollPrivacyRequest(res.request_id, opts)
  if (!isSuccessPrivacyStatus(final.status)) {
    throw new Error(final.error_msg || `export_${final.status || 'failed'}`)
  }

  let data: unknown = null
  if (final.payload) {
    try {
      data = JSON.parse(final.payload)
    } catch {
      data = final.payload
    }
  }
  if (data == null) {
    throw new Error('export_empty_payload')
  }
  return { requestId: res.request_id, status: String(final.status), data }
}

/**
 * 发起删除并轮询到终态；非 completed 抛错。
 */
export async function deleteWithStatus(
  opts: RequestDeleteOptions & PollOptions = {},
): Promise<{ requestId: string; status: string; scope: string }> {
  const res = await requestPrivacyDelete(opts)
  if (!res?.request_id) {
    throw new Error('delete_missing_request_id')
  }

  let status = String(res.status || '')
  if (!isTerminalPrivacyStatus(status)) {
    const final = await pollPrivacyRequest(res.request_id, opts)
    status = String(final.status || '')
    if (!isSuccessPrivacyStatus(status)) {
      throw new Error(final.error_msg || `delete_${status || 'failed'}`)
    }
  } else if (!isSuccessPrivacyStatus(status)) {
    throw new Error(res.error_msg || `delete_${status || 'failed'}`)
  }

  return {
    requestId: res.request_id,
    status,
    scope: res.scope || opts.scope || 'device',
  }
}
