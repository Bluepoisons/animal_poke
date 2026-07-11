/**
 * PrivacyCenter UI — AP-068
 * 不在服务端/剪贴板失败时展示成功。
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/react'
import { AppProviders } from '../providers/AppProviders'
import SettingsScreen from '../settings/SettingsScreen'
import { grantConsent } from '../compliance'
import * as privacyApi from './api'
import * as clipboard from './clipboard'

function renderSettings(onToast = vi.fn()) {
  return render(
    <AppProviders>
      <SettingsScreen onToast={onToast} />
    </AppProviders>,
  )
}

describe('PrivacyCenter in SettingsScreen', () => {
  beforeEach(() => {
    localStorage.clear()
    grantConsent(['photo', 'location'])
  })
  afterEach(() => {
    cleanup()
    vi.restoreAllMocks()
  })

  it('renders privacy center with four scopes on production settings path', () => {
    renderSettings()
    expect(screen.getByTestId('privacy-center')).toBeTruthy()
    expect(screen.getByTestId('privacy-scope-photo')).toBeTruthy()
    expect(screen.getByTestId('privacy-scope-location')).toBeTruthy()
    expect(screen.getByTestId('privacy-scope-precise_location')).toBeTruthy()
    expect(screen.getByTestId('privacy-scope-analytics')).toBeTruthy()
    expect(screen.getByTestId('privacy-export-server')).toBeTruthy()
    expect(screen.getByTestId('privacy-delete-device')).toBeTruthy()
    expect(screen.getByTestId('privacy-delete-account')).toBeTruthy()
    expect(screen.getByTestId('privacy-delete-local')).toBeTruthy()
  })

  it('does not toast success when server export fails', async () => {
    const onToast = vi.fn()
    vi.spyOn(privacyApi, 'exportWithStatus').mockRejectedValue(new Error('network_down'))
    renderSettings(onToast)
    fireEvent.click(screen.getByTestId('privacy-export-server'))
    await waitFor(() => {
      expect(onToast).toHaveBeenCalled()
    })
    const msgs = onToast.mock.calls.map((c) => String(c[0]))
    expect(msgs.some((m) => /network_down|失败|failed|Export failed|导出失败/i.test(m))).toBe(true)
    expect(msgs.some((m) => /已复制|copied|已下载|downloaded/i.test(m))).toBe(false)
  })

  it('does not toast success when clipboard fails after export', async () => {
    const onToast = vi.fn()
    vi.spyOn(privacyApi, 'exportWithStatus').mockResolvedValue({
      requestId: 'r1',
      status: 'completed',
      data: { animals: [] },
    })
    vi.spyOn(clipboard, 'exportTextPayload').mockResolvedValue({
      ok: false,
      error: 'clipboard_unavailable',
    })
    renderSettings(onToast)
    fireEvent.click(screen.getByTestId('privacy-export-server'))
    await waitFor(() => {
      expect(onToast).toHaveBeenCalled()
    })
    const msgs = onToast.mock.calls.map((c) => String(c[0]))
    expect(msgs.some((m) => /剪贴板|clipboard|download failed|下载失败/i.test(m))).toBe(true)
    expect(msgs.some((m) => /服务端导出已复制|Server export copied/i.test(m))).toBe(false)
  })

  it('does not toast success when device delete fails', async () => {
    const onToast = vi.fn()
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    vi.spyOn(privacyApi, 'deleteWithStatus').mockRejectedValue(new Error('delete_failed'))
    renderSettings(onToast)
    fireEvent.click(screen.getByTestId('privacy-delete-device'))
    await waitFor(() => {
      expect(onToast).toHaveBeenCalled()
    })
    const msgs = onToast.mock.calls.map((c) => String(c[0]))
    expect(msgs.some((m) => /delete_failed|失败|failed/i.test(m))).toBe(true)
    expect(msgs.some((m) => /本设备数据已删除|Device data deleted/i.test(m))).toBe(false)
  })

  it('toasts success only after server export + clipboard ok', async () => {
    const onToast = vi.fn()
    vi.spyOn(privacyApi, 'exportWithStatus').mockResolvedValue({
      requestId: 'r2',
      status: 'completed',
      data: { animals: [1] },
    })
    vi.spyOn(clipboard, 'exportTextPayload').mockResolvedValue({
      ok: true,
      method: 'clipboard',
    })
    renderSettings(onToast)
    fireEvent.click(screen.getByTestId('privacy-export-server'))
    await waitFor(() => {
      expect(onToast).toHaveBeenCalled()
    })
    const msgs = onToast.mock.calls.map((c) => String(c[0]))
    expect(msgs.some((m) => /服务端导出已复制|Server export copied/i.test(m))).toBe(true)
  })
})
