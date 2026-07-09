import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { reportError, flushQueue, _resetForTesting } from './reporter'

describe('reporter', () => {
  beforeEach(() => {
    _resetForTesting()
    vi.restoreAllMocks()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('sends error report when online', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response('{}', { status: 200 })
    )
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
    expect(fetchSpy).toHaveBeenCalledTimes(1)
    expect(fetchSpy.mock.calls[0][0]).toBe('/api/v1/errors/report')
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

    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response('{}', { status: 200 })
    )
    await flushQueue()
    expect(fetchSpy).toHaveBeenCalledTimes(1)
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

    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response('{}', { status: 200 })
    )
    await flushQueue()
    // Queue capped at 20
    expect(fetchSpy).toHaveBeenCalledTimes(20)
  })

  it('retries on 5xx server error', async () => {
    let calls = 0
    vi.spyOn(globalThis, 'fetch').mockImplementation(async () => {
      calls++
      if (calls < 2) return new Response('err', { status: 500 })
      return new Response('{}', { status: 200 })
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
    expect(calls).toBeGreaterThanOrEqual(2)
  })
})
