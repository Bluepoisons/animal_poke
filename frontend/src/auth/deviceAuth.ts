/**
 * 设备注册 + Token 生命周期（AP-078）
 * 仅存设备 ID、installation_secret、access token 与 refresh token，永不存第三方 Key。
 * 401 时 singleflight 优先 refresh 轮换；仅重放安全/幂等方法，避免无限循环。
 */
import type { RequestOptions } from '../api/client'
import { apiRequest, ApiError } from '../api/client'

const DEVICE_ID_KEY = 'ap_device_id'
const TOKEN_KEY = 'ap_access_token'
const TOKEN_EXP_KEY = 'ap_token_expires_at'
const INSTALL_SECRET_KEY = 'ap_installation_secret'
/** 客户端持有的 refresh 明文；命名避免与服务端哈希混淆 */
export const REFRESH_TOKEN_KEY = 'ap_refresh_token_hash_client'
const ACCOUNT_ID_KEY = 'ap_account_id'

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

export function readRefreshToken(): string | null {
  return storage().getItem(REFRESH_TOKEN_KEY)
}

export function persistRefreshToken(refresh?: string | null, accountId?: string | null): void {
  if (refresh) storage().setItem(REFRESH_TOKEN_KEY, refresh)
  if (accountId) storage().setItem(ACCOUNT_ID_KEY, accountId)
}

export function clearRefreshToken(): void {
  storage().removeItem(REFRESH_TOKEN_KEY)
}

export function clearAuth(): void {
  storage().removeItem(TOKEN_KEY)
  storage().removeItem(TOKEN_EXP_KEY)
  // 保留 device_id、installation_secret、refresh（除非显式 clearRefreshToken）
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

/** GET/HEAD/OPTIONS 或显式 allowRetry / Idempotency-Key 才允许 401 后重放 */
export function isSafeToReplay(
  method?: string,
  allowRetry?: boolean,
  idempotencyKey?: string,
): boolean {
  const m = (method || 'GET').toUpperCase()
  if (['GET', 'HEAD', 'OPTIONS'].includes(m)) return true
  if (allowRetry) return true
  if (idempotencyKey) return true
  return false
}

type DeviceAuthResponse = {
  token: string
  expires_at: string
  token_type?: string
  installation_secret?: string
}

type RefreshAuthResponse = {
  token: string
  expires_at: string
  token_type?: string
  refresh_token?: string
  account_id?: string
}

/** 向后端注册/刷新设备 Token（游客路径） */
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

/** 使用 refresh_token 轮换 access（AP-078）；失败抛错由调用方决定回退 */
export async function refreshWithToken(signal?: AbortSignal): Promise<AuthState> {
  const refresh = readRefreshToken()
  if (!refresh) {
    throw new ApiError(401, { error: 'no refresh token', reason_code: 'refresh_token_missing' })
  }
  const deviceId = getOrCreateDeviceId()
  const res = await apiRequest<RefreshAuthResponse>({
    method: 'POST',
    path: '/api/v1/auth/refresh',
    body: JSON.stringify({
      refresh_token: refresh,
      device_id: deviceId,
    }),
    signal,
    // refresh 自身禁止自动重试，避免放大 rotate 竞态
    allowRetry: false,
  })
  const state: AuthState = {
    deviceId,
    token: res.token,
    expiresAt: res.expires_at,
  }
  persistAuth(state)
  if (res.refresh_token) {
    storage().setItem(REFRESH_TOKEN_KEY, res.refresh_token)
  }
  if (res.account_id) {
    storage().setItem(ACCOUNT_ID_KEY, res.account_id)
  }
  return state
}

// singleflight：设备注册 / refresh 各一条
let inflight: Promise<AuthState> | null = null
let refreshInflight: Promise<AuthState> | null = null

/**
 * 确保有效 access token。
 * 有 refresh 时优先 singleflight 轮换；失败则清 refresh 并回退设备注册。
 */
export async function ensureAuth(signal?: AbortSignal): Promise<AuthState> {
  const existing = readStoredAuth()
  if (existing && !isExpired(existing.expiresAt)) {
    return existing
  }

  if (readRefreshToken()) {
    if (refreshInflight) return refreshInflight
    refreshInflight = refreshWithToken(signal)
      .catch(async () => {
        // refresh 失败：清空绑定凭证，回退游客设备续签（避免无限 refresh 循环）
        clearRefreshToken()
        clearAuth()
        return registerDevice(signal)
      })
      .finally(() => {
        refreshInflight = null
      })
    return refreshInflight
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
 * 带自动鉴权的 API 调用：
 * - 401 时 singleflight 续签一次
 * - 仅对安全/幂等方法重放；非幂等写请求不自动重放，避免副作用
 * - 不会对同一请求无限循环
 */
export async function authedRequest<T = unknown>(
  opts: Omit<RequestOptions, 'token'> & { token?: string | null },
): Promise<T> {
  const token = opts.token ?? (await getAccessToken(opts.signal))
  try {
    return await apiRequest<T>({ ...opts, token })
  } catch (e) {
    if (!(e instanceof ApiError) || e.status !== 401) {
      throw e
    }
    // 防止对 refresh/device 端点自身 401 再入
    const path = opts.path || ''
    if (path.includes('/auth/refresh') || path.includes('/auth/device')) {
      throw e
    }

    // 强制续签一次（singleflight）
    clearAuth()
    let fresh: AuthState
    try {
      fresh = await ensureAuth(opts.signal)
    } catch {
      throw e
    }

    if (!isSafeToReplay(opts.method, opts.allowRetry, opts.idempotencyKey)) {
      // 已刷新凭证，但不重放非幂等写
      throw e
    }
    return apiRequest<T>({ ...opts, token: fresh.token })
  }
}

export function __resetAuthForTests(): void {
  inflight = null
  refreshInflight = null
  clearAuth()
  clearRefreshToken()
  storage().removeItem(DEVICE_ID_KEY)
  storage().removeItem(INSTALL_SECRET_KEY)
  storage().removeItem(ACCOUNT_ID_KEY)
}
