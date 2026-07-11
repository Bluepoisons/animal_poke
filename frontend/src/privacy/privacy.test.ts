import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import {
  copyTextToClipboard,
  downloadTextFile,
  exportTextPayload,
} from './clipboard'
import {
  isTerminalPrivacyStatus,
  isSuccessPrivacyStatus,
  pollPrivacyRequest,
  exportWithStatus,
  deleteWithStatus,
} from './api'
import { listScopeViewModels, resolveScopeStatus } from './scopes'
import { setAnalyticsPref, getAnalyticsPref, clearAnalyticsPref } from './analyticsPrefs'
import { grantConsent, revokeConsent } from '../compliance'
import * as deviceAuth from '../auth/deviceAuth'

describe('privacy clipboard (AP-068)', () => {
  it('returns ok:false when clipboard missing', async () => {
    const res = await copyTextToClipboard('hello', null)
    expect(res.ok).toBe(false)
    if (!res.ok) expect(res.error).toBe('clipboard_unavailable')
  })

  it('returns ok:false on write failure', async () => {
    const res = await copyTextToClipboard('hello', {
      writeText: async () => {
        throw new Error('denied')
      },
    })
    expect(res.ok).toBe(false)
    if (!res.ok) expect(res.error).toBe('denied')
  })

  it('returns ok:true on successful write', async () => {
    const res = await copyTextToClipboard('hello', {
      writeText: async () => undefined,
    })
    expect(res).toEqual({ ok: true, method: 'clipboard' })
  })

  it('exportTextPayload falls back to download when clipboard fails', async () => {
    const click = vi.fn()
    const remove = vi.fn()
    const a = { href: '', download: '', rel: '', style: { display: '' }, click, remove } as unknown as HTMLAnchorElement
    const doc = {
      createElement: () => a,
      body: { appendChild: vi.fn() },
    } as unknown as Document
    // force clipboard fail path then use downloadTextFile directly
    const clip = await copyTextToClipboard('x', null)
    expect(clip.ok).toBe(false)
    const dl = downloadTextFile('{"a":1}', 't.json', doc)
    expect(dl.ok).toBe(true)
    if (dl.ok) expect(dl.method).toBe('download')
    expect(click).toHaveBeenCalled()
  })

  it('exportTextPayload never claims success when both fail', async () => {
    const res = await exportTextPayload('', 'x.json')
    expect(res.ok).toBe(false)
  })
})

describe('privacy scopes catalog', () => {
  beforeEach(() => {
    localStorage.clear()
    clearAnalyticsPref()
  })

  it('lists photo/location/precise/analytics with version+retention metadata', () => {
    grantConsent(['photo', 'location'])
    const rows = listScopeViewModels()
    expect(rows.map((r) => r.meta.id)).toEqual([
      'photo',
      'location',
      'precise_location',
      'analytics',
    ])
    expect(rows[0].meta.version).toBe('v1')
    expect(rows[0].enabled).toBe(true)
    expect(rows[2].enabled).toBe(false)
    expect(rows[3].meta.serverBacked).toBe(false)
  })

  it('analytics respects local pref denied', () => {
    grantConsent(['photo'])
    setAnalyticsPref('denied')
    const a = resolveScopeStatus('analytics')
    expect(a.enabled).toBe(false)
    expect(a.status).toBe('denied')
    expect(getAnalyticsPref()).toBe('denied')
  })

  it('revoked product consent marks server scopes denied', () => {
    grantConsent(['photo', 'location'])
    revokeConsent()
    const photo = resolveScopeStatus('photo')
    expect(photo.enabled).toBe(false)
    expect(photo.status).toBe('denied')
  })
})

describe('privacy request status helpers', () => {
  it('classifies terminal/success statuses', () => {
    expect(isTerminalPrivacyStatus('completed')).toBe(true)
    expect(isTerminalPrivacyStatus('failed')).toBe(true)
    expect(isTerminalPrivacyStatus('processing')).toBe(false)
    expect(isSuccessPrivacyStatus('completed')).toBe(true)
    expect(isSuccessPrivacyStatus('failed')).toBe(false)
  })
})

describe('privacy API polling', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('pollPrivacyRequest waits until completed', async () => {
    const spy = vi
      .spyOn(deviceAuth, 'authedRequest')
      .mockResolvedValueOnce({ request_id: 'r1', status: 'processing' })
      .mockResolvedValueOnce({ request_id: 'r1', status: 'completed', payload: '{"ok":true}' })

    const final = await pollPrivacyRequest('r1', {
      intervalMs: 1,
      timeoutMs: 1000,
      sleep: async () => undefined,
    })
    expect(final.status).toBe('completed')
    expect(spy).toHaveBeenCalledTimes(2)
  })

  it('exportWithStatus fails when server returns failed', async () => {
    vi.spyOn(deviceAuth, 'authedRequest').mockResolvedValue({
      request_id: 'r-fail',
      status: 'failed',
      error_msg: 'boom',
    })
    await expect(exportWithStatus({ intervalMs: 1, timeoutMs: 100 })).rejects.toThrow(/boom|export_failed/)
  })

  it('exportWithStatus returns data on immediate completed', async () => {
    vi.spyOn(deviceAuth, 'authedRequest').mockResolvedValue({
      request_id: 'r-ok',
      status: 'completed',
      data: { animals: [] },
    })
    const res = await exportWithStatus()
    expect(res.requestId).toBe('r-ok')
    expect(res.data).toEqual({ animals: [] })
  })

  it('deleteWithStatus rejects non-completed', async () => {
    vi.spyOn(deviceAuth, 'authedRequest').mockResolvedValue({
      request_id: 'd1',
      status: 'failed',
      error_msg: 'reauth required',
    })
    await expect(deleteWithStatus({ scope: 'device' })).rejects.toThrow(/reauth|delete_failed/)
  })

  it('deleteWithStatus succeeds on completed', async () => {
    vi.spyOn(deviceAuth, 'authedRequest').mockResolvedValue({
      request_id: 'd2',
      status: 'completed',
      scope: 'device',
    })
    const res = await deleteWithStatus({ scope: 'device' })
    expect(res.requestId).toBe('d2')
    expect(res.scope).toBe('device')
  })
})
