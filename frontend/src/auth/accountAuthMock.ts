/**
 * Development/test Mock OAuth helpers (AP-063).
 * Not imported by production account UI; unit tests may import directly.
 */
import { apiRequest } from '../api/client'
import {
  ensureAuth,
  getOrCreateDeviceId,
  type AuthState,
} from './deviceAuth'
import { isAuthMockOAuthEnabled } from './authProviders'

const ACCOUNT_ID_KEY = 'ap_account_id'
const REFRESH_TOKEN_KEY = 'ap_refresh_token_hash_client'

type AccountAuthResponse = {
  token: string
  expires_at: string
  token_type?: string
  account_id?: string
  refresh_token?: string
  guest?: boolean
  merge?: {
    animals_moved?: number
    entitlements_merged?: number
    entitlements_moved?: number
  }
}

export type MockBindResult = AuthState & {
  accountId: string
  refreshToken?: string
  merge?: AccountAuthResponse['merge']
}

function persistBound(state: AuthState, accountId?: string, refresh?: string) {
  localStorage.setItem('ap_device_id', state.deviceId)
  localStorage.setItem('ap_access_token', state.token)
  localStorage.setItem('ap_token_expires_at', state.expiresAt)
  if (accountId) localStorage.setItem(ACCOUNT_ID_KEY, accountId)
  if (refresh) localStorage.setItem(REFRESH_TOKEN_KEY, refresh)
}

function assertMockAllowed() {
  if (!isAuthMockOAuthEnabled()) {
    throw new Error('mock oauth is not available in this build')
  }
}

export async function bindMockOAuth(
  subject: string,
  token: string,
  displayName?: string,
  signal?: AbortSignal,
): Promise<MockBindResult> {
  assertMockAllowed()
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

export async function loginMockOAuth(
  subject: string,
  token: string,
  signal?: AbortSignal,
): Promise<MockBindResult> {
  assertMockAllowed()
  const deviceId = getOrCreateDeviceId()
  const installationSecret =
    typeof localStorage !== 'undefined'
      ? localStorage.getItem('ap_installation_secret') || undefined
      : undefined
  const res = await apiRequest<AccountAuthResponse>({
    method: 'POST',
    path: '/api/v1/auth/login',
    body: JSON.stringify({
      device_id: deviceId,
      provider: 'mock_oauth',
      oauth_subject: subject,
      oauth_token: token,
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
