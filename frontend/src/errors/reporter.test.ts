import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { reportError, flushQueue, _resetForTesting } from './reporter'
import { __resetAuthForTests } from '../auth/deviceAuth'

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

function mockFetchAuthed() {
  return vi.spyOn(globalThis, 'fetch').mockImplementation(async (input: RequestInfo | URL) => {
    const url = String(input)
    if (url.includes('/auth/device')) {
      return jsonResponse({
        token: 'tok-test',
        expires_at: new Date(Date.now() + 3600_000).toISOString(),
      })
    }
    if (url.includes('/errors/report')) {
      return jsonResponse({ accepted: true }, 202)
    }
    return jsonResponse({ error: 'not found' }, 404)
  })
}

describe('reporter', () => {
  beforeEach(() => {
    _resetForTesting()
    __resetAuthForTests()
    vi.restoreAllMocks()
  })

  afterEach(() => {
    vi.restoreAllMocks()
    __resetAuthForTests()
  })

  it('sends error report when online', async () => {
    const fetchSpy = mockFetchAuthed()
    await reportError({
      id: 'test-1',
      type: 'window',
      name: 'Error',
      message: 'test',
      timestamp: Date.now(),
      appVersion: '1.0.0',
      userAgent: 'test',
      online: true,
    })
    const urls = fetchSpy.mock.calls.map((c) => String(c[0]))
    expect(urls.some((u) => u.includes('/errors/report'))).toBe(true)
  })

  it('enqueues when offline', async () => {
    vi.stubGlobal('navigator', { onLine: false })
    const fetchSpy = vi.spyOn(globalThis, 'fetch')
    await reportError({
      id: 'test-2',
      type: 'window',
      name: 'Error',
      message: 'offline test',
      timestamp: Date.now(),
      appVersion: '1.0.0',
      userAgent: 'test',
      online: false,
    })
    expect(fetchSpy).not.toHaveBeenCalled()
    vi.unstubAllGlobals()
  })

  it('flushQueue sends queued reports', async () => {
    vi.stubGlobal('navigator', { onLine: false })
    await reportError({
      id: 'test-3',
      type: 'window',
      name: 'Error',
      message: 'queued',
      timestamp: Date.now(),
      appVersion: '1.0.0',
      userAgent: 'test',
      online: false,
    })
    vi.unstubAllGlobals()

    const fetchSpy = mockFetchAuthed()
    await flushQueue()
    const urls = fetchSpy.mock.calls.map((c) => String(c[0]))
    expect(urls.some((u) => u.includes('/errors/report'))).toBe(true)
  })

  it('caps queue at 20 items', async () => {
    vi.stubGlobal('navigator', { onLine: false })
    for (let i = 0; i < 25; i++) {
      await reportError({
        id: `test-${i}`,
        type: 'window',
        name: 'Error',
        message: `err ${i}`,
        timestamp: Date.now(),
        appVersion: '1.0.0',
        userAgent: 'test',
        online: false,
      })
    }
    vi.unstubAllGlobals()

    let reportCalls = 0
    vi.spyOn(globalThis, 'fetch').mockImplementation(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/auth/device')) {
        return jsonResponse({
          token: 'tok-test',
          expires_at: new Date(Date.now() + 3600_000).toISOString(),
        })
      }
      if (url.includes('/errors/report')) {
        reportCalls++
        return jsonResponse({ accepted: true }, 202)
      }
      return jsonResponse({}, 404)
    })
    await flushQueue()
    expect(reportCalls).toBe(20)
  })

  it('retries on 5xx server error', async () => {
    let reportCalls = 0
    vi.spyOn(globalThis, 'fetch').mockImplementation(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/auth/device')) {
        return jsonResponse({
          token: 'tok-test',
          expires_at: new Date(Date.now() + 3600_000).toISOString(),
        })
      }
      if (url.includes('/errors/report')) {
        reportCalls++
        if (reportCalls < 2) return new Response('err', { status: 500 })
        return jsonResponse({ accepted: true }, 202)
      }
      return jsonResponse({}, 404)
    })
    await reportError({
      id: 'test-retry',
      type: 'window',
      name: 'Error',
      message: 'retry test',
      timestamp: Date.now(),
      appVersion: '1.0.0',
      userAgent: 'test',
      online: true,
    })
    expect(reportCalls).toBeGreaterThanOrEqual(2)
  })
})
