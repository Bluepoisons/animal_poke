import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { fetchWithRetry, FetchTimeoutError, FetchAbortedError } from './fetchWithRetry'

describe('fetchWithRetry', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })
  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('returns successful response', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(new Response('ok', { status: 200 })),
    )
    const p = fetchWithRetry('/x', { method: 'GET' }, { retries: 1, timeout: 1000 })
    await expect(p).resolves.toMatchObject({ status: 200 })
  })

  it('retries idempotent 5xx then succeeds', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(new Response('err', { status: 500 }))
      .mockResolvedValueOnce(new Response('ok', { status: 200 }))
    vi.stubGlobal('fetch', fetchMock)

    const p = fetchWithRetry('/x', { method: 'GET' }, { retries: 2, retryDelay: 10, timeout: 5000 })
    const done = p.then((r) => r.status)
    await vi.advanceTimersByTimeAsync(50)
    await expect(done).resolves.toBe(200)
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })

  it('does not retry non-idempotent POST by default', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response('err', { status: 500 }))
    vi.stubGlobal('fetch', fetchMock)
    const p = fetchWithRetry('/x', { method: 'POST', body: '{}' }, { retries: 3, retryDelay: 10 })
    await expect(p).resolves.toMatchObject({ status: 500 })
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('throws FetchTimeoutError on internal timeout', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation((_u, init: RequestInit) => {
        return new Promise((_resolve, reject) => {
          init.signal?.addEventListener('abort', () => {
            const e = new Error('aborted')
            e.name = 'AbortError'
            reject(e)
          })
        })
      }),
    )
    const p = fetchWithRetry('/x', { method: 'GET' }, { retries: 0, timeout: 20 })
    const assertion = expect(p).rejects.toBeInstanceOf(FetchTimeoutError)
    await vi.advanceTimersByTimeAsync(30)
    await assertion
  })

  it('throws FetchAbortedError when caller aborts', async () => {
    const parent = new AbortController()
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation((_u, init: RequestInit) => {
        return new Promise((_resolve, reject) => {
          init.signal?.addEventListener('abort', () => {
            const e = new Error('aborted')
            e.name = 'AbortError'
            reject(e)
          })
        })
      }),
    )
    const p = fetchWithRetry('/x', { method: 'GET' }, { retries: 0, timeout: 10_000, signal: parent.signal })
    parent.abort()
    await expect(p).rejects.toBeInstanceOf(FetchAbortedError)
  })
})
