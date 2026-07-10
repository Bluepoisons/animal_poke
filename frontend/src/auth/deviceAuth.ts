/**
 * 设备注册 + Token 生命周期
 * 仅存设备 ID、installation_secret 与后端签发 Token，永不存第三方 Key。
 */

import type { RequestOptions } from '../api/client'
import { apiRequest, ApiError } from '../api/client'

const DEVICE_ID_KEY = 'ap_device_id'
const TOKEN_KEY = 'ap_access_token'
const TOKEN_EXP_KEY = 'ap_token_expires_at'
const INSTALL_SECRET_KEY = 'ap_installation_secret'

export type AuthState = {
  deviceId: string
  token: string
  expiresAt: string
}

function storage(): Storage {
  return localStorage
}

export function getOrCreateDeviceId(): string {
  let id = storage().getItem(DEVICE_ID_KEY)
  if (id && id.length >= 8) return id
  id =
    typeof crypto !== 'undefined' && 'randomUUID' in crypto
      ? crypto.randomUUID()
      : `dev-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`
  storage().setItem(DEVICE_ID_KEY, id)
  return id
}

export function readStoredAuth(): AuthState | null {
  const deviceId = storage().getItem(DEVICE_ID_KEY)
  const token = storage().getItem(TOKEN_KEY)
  const expiresAt = storage().getItem(TOKEN_EXP_KEY)
  if (!deviceId || !token || !expiresAt) return null
  return { deviceId, token, expiresAt }
}

export function clearAuth(): void {
  storage().removeItem(TOKEN_KEY)
  storage().removeItem(TOKEN_EXP_KEY)
  // 保留 device_id 与 installation_secret 以便稳定身份与续签
}

function persistAuth(state: AuthState): void {
  storage().setItem(DEVICE_ID_KEY, state.deviceId)
  storage().setItem(TOKEN_KEY, state.token)
  storage().setItem(TOKEN_EXP_KEY, state.expiresAt)
}

function isExpired(expiresAt: string, skewMs = 30_000): boolean {
  const t = Date.parse(expiresAt)
  if (Number.isNaN(t)) return true
  return Date.now() + skewMs >= t
}

type DeviceAuthResponse = {
  token: string
  expires_at: string
  token_type?: string
  installation_secret?: string
}

/** 向后端注册/刷新设备 Token */
export async function registerDevice(signal?: AbortSignal): Promise<AuthState> {
  const deviceId = getOrCreateDeviceId()
  const installationSecret = storage().getItem(INSTALL_SECRET_KEY)
  const body: { device_id: string; installation_secret?: string } = {
    device_id: deviceId,
  }
  if (installationSecret) {
    body.installation_secret = installationSecret
  }
  const res = await apiRequest<DeviceAuthResponse>({
    method: 'POST',
    path: '/api/v1/auth/device',
    body: JSON.stringify(body),
    signal,
    // 注册可带幂等键
    idempotencyKey: `auth-device-${deviceId}`,
    allowRetry: true,
  })
  if (res.installation_secret) {
    storage().setItem(INSTALL_SECRET_KEY, res.installation_secret)
  }
  const state: AuthState = {
    deviceId,
    token: res.token,
    expiresAt: res.expires_at,
  }
  persistAuth(state)
  return state
}

// singleflight 续签
let inflight: Promise<AuthState> | null = null

export async function ensureAuth(signal?: AbortSignal): Promise<AuthState> {
  const existing = readStoredAuth()
  if (existing && !isExpired(existing.expiresAt)) {
    return existing
  }
  if (inflight) return inflight
  inflight = registerDevice(signal).finally(() => {
    inflight = null
  })
  return inflight
}

export async function getAccessToken(signal?: AbortSignal): Promise<string> {
  const auth = await ensureAuth(signal)
  return auth.token
}

/**
 * 带自动鉴权的 API 调用：401 时 singleflight 续签一次后重试。
 */
export async function authedRequest<T = unknown>(
  opts: Omit<RequestOptions, 'token'> & { token?: string | null },
): Promise<T> {
  const token = opts.token ?? (await getAccessToken(opts.signal))
  try {
    return await apiRequest<T>({ ...opts, token })
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) {
      clearAuth()
      const fresh = await ensureAuth(opts.signal)
      return apiRequest<T>({ ...opts, token: fresh.token })
    }
    throw e
  }
}

export function __resetAuthForTests(): void {
  inflight = null
  clearAuth()
  storage().removeItem(DEVICE_ID_KEY)
  storage().removeItem(INSTALL_SECRET_KEY)
}
