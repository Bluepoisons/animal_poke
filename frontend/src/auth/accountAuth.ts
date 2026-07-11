/**
 * 账号绑定 / 登录恢复 / 设备管理（AP-055 / AP-078）
 * 凭证仅发送到后端；refresh_token 若返回则存本地（服务端只存哈希）。
 */

import { apiRequest } from '../api/client'
import {
  ensureAuth,
  getOrCreateDeviceId,
  clearAuth,
  __resetAuthForTests,
  persistRefreshToken,
  clearRefreshToken,
  type AuthState,
} from './deviceAuth'

const ACCOUNT_ID_KEY = 'ap_account_id'

export type AccountInfo = {
  guest: boolean
  accountId?: string
  displayName?: string
  status?: string
  deviceId?: string
  bindings?: Array<{ provider: string; providerSubject: string; verified: boolean }>
}

export type DeviceInfo = {
  deviceId: string
  deviceLabel: string
  status: string
  linkedAt?: string
  lastSeenAt?: string
  revokedAt?: string
  current?: boolean
}

export type BindResult = AuthState & {
  accountId: string
  refreshToken?: string
  merge?: {
    animals_moved?: number
    entitlements_merged?: number
    entitlements_moved?: number
  }
}

type AccountAuthResponse = {
  token: string
  expires_at: string
  token_type?: string
  account_id?: string
  refresh_token?: string
  guest?: boolean
  merge?: BindResult['merge']
}

function storage(): Storage {
  return localStorage
}

function persistBound(state: AuthState, accountId?: string, refresh?: string) {
  storage().setItem('ap_device_id', state.deviceId)
  storage().setItem('ap_access_token', state.token)
  storage().setItem('ap_token_expires_at', state.expiresAt)
  if (accountId) storage().setItem(ACCOUNT_ID_KEY, accountId)
  if (refresh) persistRefreshToken(refresh, accountId)
}

export function readAccountId(): string | null {
  return storage().getItem(ACCOUNT_ID_KEY)
}

export function clearAccountLocal(): void {
  storage().removeItem(ACCOUNT_ID_KEY)
  clearRefreshToken()
  clearAuth()
}

/** 绑定邮箱 */
export async function bindEmail(
  email: string,
  password: string,
  displayName?: string,
  signal?: AbortSignal,
): Promise<BindResult> {
  const auth = await ensureAuth(signal)
  const res = await apiRequest<AccountAuthResponse>({
    method: 'POST',
    path: '/api/v1/auth/bind',
    token: auth.token,
    body: JSON.stringify({
      provider: 'email',
      email,
      password,
      display_name: displayName,
    }),
    signal,
  })
  const state: AuthState = {
    deviceId: auth.deviceId,
    token: res.token,
    expiresAt: res.expires_at,
  }
  persistBound(state, res.account_id, res.refresh_token)
  return {
    ...state,
    accountId: res.account_id || '',
    refreshToken: res.refresh_token,
    merge: res.merge,
  }
}

export async function loginEmail(
  email: string,
  password: string,
  signal?: AbortSignal,
): Promise<BindResult> {
  const deviceId = getOrCreateDeviceId()
  // AP-076: 若本地持有 installation_secret，登录时一并提交以证明设备所有权（认领游客资产/复活设备）
  const installationSecret = storage().getItem('ap_installation_secret') || undefined
  const res = await apiRequest<AccountAuthResponse>({
    method: 'POST',
    path: '/api/v1/auth/login',
    body: JSON.stringify({
      device_id: deviceId,
      provider: 'email',
      email,
      password,
      ...(installationSecret ? { installation_secret: installationSecret } : {}),
    }),
    signal,
  })
  const state: AuthState = {
    deviceId,
    token: res.token,
    expiresAt: res.expires_at,
  }
  persistBound(state, res.account_id, res.refresh_token)
  return {
    ...state,
    accountId: res.account_id || '',
    refreshToken: res.refresh_token,
    merge: res.merge,
  }
}

export async function logoutAccount(signal?: AbortSignal): Promise<void> {
  try {
    const auth = await ensureAuth(signal)
    await apiRequest({
      method: 'POST',
      path: '/api/v1/auth/logout',
      token: auth.token,
      signal,
    })
  } catch {
    // 本地仍清理
  }
  clearAccountLocal()
  // 保留 device_id
  const id = storage().getItem('ap_device_id')
  clearAuth()
  if (id) storage().setItem('ap_device_id', id)
}

export async function fetchAccount(signal?: AbortSignal): Promise<AccountInfo> {
  const auth = await ensureAuth(signal)
  const res = await apiRequest<{
    guest: boolean
    account_id?: string
    display_name?: string
    status?: string
    device_id?: string
    bindings?: Array<{ provider: string; provider_subject?: string; verified?: boolean }>
  }>({
    method: 'GET',
    path: '/api/v1/auth/account',
    token: auth.token,
    signal,
  })
  if (res.account_id) storage().setItem(ACCOUNT_ID_KEY, res.account_id)
  return {
    guest: !!res.guest,
    accountId: res.account_id,
    displayName: res.display_name,
    status: res.status,
    deviceId: res.device_id,
    bindings: (res.bindings || []).map((b) => ({
      provider: b.provider,
      providerSubject: b.provider_subject || '',
      verified: !!b.verified,
    })),
  }
}

export async function listDevices(signal?: AbortSignal): Promise<DeviceInfo[]> {
  const auth = await ensureAuth(signal)
  const res = await apiRequest<{
    items?: Array<{
      device_id: string
      device_label?: string
      status: string
      linked_at?: string
      last_seen_at?: string
      revoked_at?: string
      current?: boolean
    }>
  }>({
    method: 'GET',
    path: '/api/v1/auth/devices',
    token: auth.token,
    signal,
  })
  return (res.items || []).map((d) => ({
    deviceId: d.device_id,
    deviceLabel: d.device_label || d.device_id,
    status: d.status,
    linkedAt: d.linked_at,
    lastSeenAt: d.last_seen_at,
    revokedAt: d.revoked_at,
    current: d.current,
  }))
}

export async function revokeDevice(deviceId: string, signal?: AbortSignal): Promise<void> {
  const auth = await ensureAuth(signal)
  await apiRequest({
    method: 'POST',
    path: '/api/v1/auth/devices/revoke',
    token: auth.token,
    body: JSON.stringify({ device_id: deviceId }),
    signal,
  })
}


/** 请求邮箱验证（反枚举） */
export async function requestEmailVerify(email: string, signal?: AbortSignal): Promise<void> {
  await apiRequest({
    method: 'POST',
    path: '/api/v1/auth/email/verify/request',
    body: JSON.stringify({ email }),
    signal,
  })
}

/** 使用令牌验证邮箱 */
export async function verifyEmail(token: string, signal?: AbortSignal): Promise<void> {
  await apiRequest({
    method: 'POST',
    path: '/api/v1/auth/email/verify',
    body: JSON.stringify({ token }),
    signal,
  })
}

/** 忘记密码（反枚举） */
export async function forgotPassword(email: string, signal?: AbortSignal): Promise<void> {
  await apiRequest({
    method: 'POST',
    path: '/api/v1/auth/password/forgot',
    body: JSON.stringify({ email }),
    signal,
  })
}

/** 使用重置令牌设置新密码 */
export async function resetPassword(
  token: string,
  newPassword: string,
  signal?: AbortSignal,
): Promise<void> {
  await apiRequest({
    method: 'POST',
    path: '/api/v1/auth/password/reset',
    body: JSON.stringify({ token, new_password: newPassword }),
    signal,
  })
}

/** 改密：成功后写入新 access/refresh */
export async function changePassword(
  currentPassword: string,
  newPassword: string,
  signal?: AbortSignal,
): Promise<BindResult> {
  const auth = await ensureAuth(signal)
  const res = await apiRequest<AccountAuthResponse>({
    method: 'POST',
    path: '/api/v1/auth/password/change',
    token: auth.token,
    body: JSON.stringify({
      current_password: currentPassword,
      new_password: newPassword,
    }),
    signal,
  })
  const state: AuthState = {
    deviceId: auth.deviceId,
    token: res.token,
    expiresAt: res.expires_at,
  }
  persistBound(state, res.account_id, res.refresh_token)
  return {
    ...state,
    accountId: res.account_id || '',
    refreshToken: res.refresh_token,
  }
}

/** 解绑 provider（需 reauth） */
export async function unbindProvider(
  provider: string,
  opts?: { subject?: string; reauthPassword?: string; reauthToken?: string },
  signal?: AbortSignal,
): Promise<void> {
  const auth = await ensureAuth(signal)
  await apiRequest({
    method: 'POST',
    path: '/api/v1/auth/unbind',
    token: auth.token,
    body: JSON.stringify({
      provider,
      subject: opts?.subject,
      reauth_password: opts?.reauthPassword,
      reauth_token: opts?.reauthToken,
    }),
    signal,
  })
}

/** 近期 re-auth 令牌 */
export async function reauth(password: string, signal?: AbortSignal): Promise<string> {
  const auth = await ensureAuth(signal)
  const res = await apiRequest<{ reauth_token: string }>({
    method: 'POST',
    path: '/api/v1/auth/reauth',
    token: auth.token,
    body: JSON.stringify({ password }),
    signal,
  })
  return res.reauth_token
}

export function __resetAccountForTests(): void {
  clearAccountLocal()
  __resetAuthForTests()
}
