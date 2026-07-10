import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import {
  getOrCreateDeviceId,
  ensureAuth,
  readStoredAuth,
  clearAuth,
  __resetAuthForTests,
  authedRequest,
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
      }),
    })
    vi.stubGlobal('fetch', fetchMock)

    const auth = await ensureAuth()
    expect(auth.token).toBe('tok-1')
    expect(readStoredAuth()?.token).toBe('tok-1')
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
    let resolveJson: (v: unknown) => void
    const jsonPromise = new Promise((r) => {
      resolveJson = r
    })
    const fetchMock = vi.fn().mockImplementation(async () => ({
      ok: true,
      status: 200,
      headers: new Headers({ 'Content-Type': 'application/json' }),
      json: () => jsonPromise,
    }))
    vi.stubGlobal('fetch', fetchMock)

    const p1 = ensureAuth()
    const p2 = ensureAuth()
    resolveJson!({
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

    // seed expired-looking by first register
    const res = await authedRequest<{ msg: string }>({ path: '/api/v1/ping' })
    expect(res.msg).toBe('pong')
    clearAuth()
  })
})
