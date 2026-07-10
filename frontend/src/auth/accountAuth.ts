/**
 * 账号绑定 / 登录恢复 / 设备管理（AP-055）
 * 凭证仅发送到后端；refresh_token 若返回则存本地（服务端只存哈希）。
 */

import { apiRequest } from '../api/client'
import {
  ensureAuth,
  getOrCreateDeviceId,
  clearAuth,
  __resetAuthForTests,
  type AuthState,
} from './deviceAuth'

const ACCOUNT_ID_KEY = 'ap_account_id'
const REFRESH_TOKEN_KEY = 'ap_refresh_token_hash_client' // 客户端持有明文 refresh 一次；命名避免与服务端混淆

export type AccountInfo = {
  guest: boolean
  accountId?: string
  displayName?: string
  status?: string
  deviceId?: string
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
  if (refresh) storage().setItem(REFRESH_TOKEN_KEY, refresh)
}

export function readAccountId(): string | null {
  return storage().getItem(ACCOUNT_ID_KEY)
}

export function clearAccountLocal(): void {
  storage().removeItem(ACCOUNT_ID_KEY)
  storage().removeItem(REFRESH_TOKEN_KEY)
  clearAuth()
}

/** 绑定 mock OAuth（开发） */
export async function bindMockOAuth(
  subject: string,
  token: string,
  displayName?: string,
  signal?: AbortSignal,
): Promise<BindResult> {
  const auth = await ensureAuth(signal)
  const res = await apiRequest<AccountAuthResponse>({
    method: 'POST',
    path: '/api/v1/auth/bind',
    token: auth.token,
    body: JSON.stringify({
      provider: 'mock_oauth',
      oauth_subject: subject,
      oauth_token: token,
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

/** 清除本地后用 mock 登录恢复 */
export async function loginMockOAuth(
  subject: string,
  token: string,
  signal?: AbortSignal,
): Promise<BindResult> {
  const deviceId = getOrCreateDeviceId()
  const res = await apiRequest<AccountAuthResponse>({
    method: 'POST',
    path: '/api/v1/auth/login',
    body: JSON.stringify({
      device_id: deviceId,
      provider: 'mock_oauth',
      oauth_subject: subject,
      oauth_token: token,
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

export async function loginEmail(
  email: string,
  password: string,
  signal?: AbortSignal,
): Promise<BindResult> {
  const deviceId = getOrCreateDeviceId()
  const res = await apiRequest<AccountAuthResponse>({
    method: 'POST',
    path: '/api/v1/auth/login',
    body: JSON.stringify({
      device_id: deviceId,
      provider: 'email',
      email,
      password,
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

export function __resetAccountForTests(): void {
  clearAccountLocal()
  __resetAuthForTests()
}
