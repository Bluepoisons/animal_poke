import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { ApiError } from '../api/client'
import {
  getOrCreateDeviceId,
  ensureAuth,
  readStoredAuth,
  clearAuth,
  __resetAuthForTests,
  authedRequest,
  persistRefreshToken,
  readRefreshToken,
  isSafeToReplay,
} from './deviceAuth'

describe('deviceAuth', () => {
  beforeEach(() => {
    __resetAuthForTests()
    vi.restoreAllMocks()
  })
  afterEach(() => {
    __resetAuthForTests()
  })

  it('creates stable device id', () => {
    const a = getOrCreateDeviceId()
    const b = getOrCreateDeviceId()
    expect(a).toBe(b)
    expect(a.length).toBeGreaterThanOrEqual(8)
  })

  it('registers device and stores token', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ 'Content-Type': 'application/json' }),
      json: async () => ({
        token: 'tok-1',
        expires_at: new Date(Date.now() + 3600_000).toISOString(),
        token_type: 'Bearer',
        installation_secret: 'inst-secret-hex-64chars-minimum-padding-aaaaaaaaaaaa',
      }),
    })
    vi.stubGlobal('fetch', fetchMock)

    const auth = await ensureAuth()
    expect(auth.token).toBe('tok-1')
    expect(readStoredAuth()?.token).toBe('tok-1')
    expect(localStorage.getItem('ap_installation_secret')).toContain('inst-secret')
    expect(fetchMock).toHaveBeenCalledTimes(1)
    const [, init] = fetchMock.mock.calls[0]
    expect(init.method).toBe('POST')
    expect(String(init.headers['Authorization'] || '')).not.toContain('tok')
  })

  it('reuses non-expired token without re-register', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ 'Content-Type': 'application/json' }),
      json: async () => ({
        token: 'tok-1',
        expires_at: new Date(Date.now() + 3600_000).toISOString(),
      }),
    })
    vi.stubGlobal('fetch', fetchMock)
    await ensureAuth()
    await ensureAuth()
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('singleflight concurrent ensureAuth', async () => {
    const { promise: jsonPromise, resolve: resolveJson } = Promise.withResolvers<unknown>()
    const fetchMock = vi.fn().mockImplementation(async () => ({
      ok: true,
      status: 200,
      headers: new Headers({ 'Content-Type': 'application/json' }),
      json: () => jsonPromise,
    }))
    vi.stubGlobal('fetch', fetchMock)

    const p1 = ensureAuth()
    const p2 = ensureAuth()
    resolveJson({
      token: 'tok-sf',
      expires_at: new Date(Date.now() + 3600_000).toISOString(),
    })
    const [a, b] = await Promise.all([p1, p2])
    expect(a.token).toBe('tok-sf')
    expect(b.token).toBe('tok-sf')
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('authedRequest refreshes once on 401', async () => {
    let call = 0
    const fetchMock = vi.fn().mockImplementation(async (url: string) => {
      call += 1
      if (String(url).includes('/auth/device')) {
        return {
          ok: true,
          status: 200,
          headers: new Headers({ 'Content-Type': 'application/json' }),
          json: async () => ({
            token: `tok-${call}`,
            expires_at: new Date(Date.now() + 3600_000).toISOString(),
          }),
        }
      }
      // first protected call 401, second ok
      if (call === 2) {
        return {
          ok: false,
          status: 401,
          headers: new Headers({ 'Content-Type': 'application/json' }),
          json: async () => ({ error: 'invalid token' }),
        }
      }
      return {
        ok: true,
        status: 200,
        headers: new Headers({ 'Content-Type': 'application/json' }),
        json: async () => ({ msg: 'pong' }),
      }
    })
    vi.stubGlobal('fetch', fetchMock)

    const res = await authedRequest<{ msg: string }>({ path: '/api/v1/ping' })
    expect(res.msg).toBe('pong')
    clearAuth()
  })

  it('ensureAuth uses refresh singleflight when refresh token present', async () => {
    localStorage.setItem('ap_device_id', 'dev-bound-1')
    localStorage.setItem('ap_access_token', 'old-tok')
    localStorage.setItem('ap_token_expires_at', new Date(Date.now() - 1000).toISOString())
    persistRefreshToken('refresh-plain-1', 'acct-1')

    const { promise: jsonPromise, resolve: resolveJson } = Promise.withResolvers<unknown>()
    const fetchMock = vi.fn().mockImplementation(async (url: string) => {
      expect(String(url)).toContain('/auth/refresh')
      return {
        ok: true,
        status: 200,
        headers: new Headers({ 'Content-Type': 'application/json' }),
        json: () => jsonPromise,
      }
    })
    vi.stubGlobal('fetch', fetchMock)

    const p1 = ensureAuth()
    const p2 = ensureAuth()
    resolveJson({
      token: 'tok-refreshed',
      expires_at: new Date(Date.now() + 3600_000).toISOString(),
      refresh_token: 'refresh-plain-2',
      account_id: 'acct-1',
    })
    const [a, b] = await Promise.all([p1, p2])
    expect(a.token).toBe('tok-refreshed')
    expect(b.token).toBe('tok-refreshed')
    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(readRefreshToken()).toBe('refresh-plain-2')
  })

  it('authedRequest with refresh does not replay unsafe POST', async () => {
    localStorage.setItem('ap_device_id', 'dev-bound-2')
    localStorage.setItem('ap_access_token', 'stale')
    localStorage.setItem(
      'ap_token_expires_at',
      new Date(Date.now() + 3600_000).toISOString(),
    )
    persistRefreshToken('refresh-plain-x')

    const urls: string[] = []
    const fetchMock = vi.fn().mockImplementation(async (url: string, init?: RequestInit) => {
      urls.push(`${init?.method || 'GET'} ${url}`)
      if (String(url).includes('/auth/refresh')) {
        return {
          ok: true,
          status: 200,
          headers: new Headers({ 'Content-Type': 'application/json' }),
          json: async () => ({
            token: 'fresh-tok',
            expires_at: new Date(Date.now() + 3600_000).toISOString(),
            refresh_token: 'refresh-plain-y',
          }),
        }
      }
      return {
        ok: false,
        status: 401,
        headers: new Headers({ 'Content-Type': 'application/json' }),
        json: async () => ({ error: 'expired' }),
      }
    })
    vi.stubGlobal('fetch', fetchMock)

    await expect(
      authedRequest({
        method: 'POST',
        path: '/api/v1/sync/animals',
        body: JSON.stringify({ items: [] }),
      }),
    ).rejects.toBeInstanceOf(ApiError)

    // 一次业务 POST + 一次 refresh；不得重放 POST
    const postCalls = urls.filter((u) => u.startsWith('POST') && u.includes('/sync/animals'))
    const refreshCalls = urls.filter((u) => u.includes('/auth/refresh'))
    expect(postCalls).toHaveLength(1)
    expect(refreshCalls).toHaveLength(1)
    expect(readStoredAuth()?.token).toBe('fresh-tok')
  })

  it('isSafeToReplay only allows safe methods by default', () => {
    expect(isSafeToReplay('GET')).toBe(true)
    expect(isSafeToReplay('POST')).toBe(false)
    expect(isSafeToReplay('POST', true)).toBe(true)
    expect(isSafeToReplay('POST', false, 'idem-1')).toBe(true)
  })
})
