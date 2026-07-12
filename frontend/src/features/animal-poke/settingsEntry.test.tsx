/**
 * AP-061: settings / account / permissions entry reachable from production shell.
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, cleanup, fireEvent, waitFor } from '@testing-library/react'
import { grantConsent, revokeConsent } from '../../compliance'
import { AppProviders } from '../../providers/AppProviders'
import AnimalPokeApp from './AnimalPokeApp'

vi.mock('../../outdoorSafety/logic', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../outdoorSafety/logic')>()
  return {
    ...actual,
    evaluateOutdoorSafety: () => ({ allowed: true, messages: [], stopFirst: false }),
  }
})

vi.mock('../../auth/accountAuth', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../auth/accountAuth')>()
  return {
    ...actual,
    fetchAccount: vi.fn().mockResolvedValue({ guest: true }),
    listDevices: vi.fn().mockResolvedValue([]),
  }
})

function mockCameraReady() {
  Object.defineProperty(navigator, 'mediaDevices', {
    configurable: true,
    value: {
      getUserMedia: vi.fn().mockResolvedValue({
        getTracks: () => [{ stop: vi.fn(), enabled: true }],
      }),
    },
  })
  HTMLVideoElement.prototype.play = vi.fn().mockResolvedValue(undefined)
  Object.defineProperty(HTMLVideoElement.prototype, 'videoWidth', {
    value: 640,
    configurable: true,
  })
  Object.defineProperty(HTMLVideoElement.prototype, 'videoHeight', {
    value: 480,
    configurable: true,
  })
  HTMLCanvasElement.prototype.toBlob = function (cb: BlobCallback) {
    cb(new Blob([new Uint8Array(2500).fill(7)], { type: 'image/jpeg' }))
  }
}

describe('AP-061 settings entry IA', () => {
  beforeEach(() => {
    localStorage.clear()
    sessionStorage.clear()
    localStorage.setItem(
      'animal-poke-onboarding-v1',
      JSON.stringify({ step: 'done', skipped: true, completedAt: Date.now() }),
    )
    grantConsent()
    mockCameraReady()
    location.hash = ''
  })
  afterEach(() => {
    cleanup()
    vi.restoreAllMocks()
    location.hash = ''
  })

  it('bottom tab always exposes settings (≤1 tap from discover)', async () => {
    render(
      <AppProviders>
        <AnimalPokeApp />
      </AppProviders>,
    )
    expect(await screen.findByTestId('discover-screen')).toBeTruthy()
    const tab = await screen.findByTestId('tab-settings')
    fireEvent.click(tab)
    expect(await screen.findByTestId('settings-screen')).toBeTruthy()
    expect(screen.getByTestId('settings-account-summary')).toBeTruthy()
    expect(screen.getByTestId('settings-guest-status').textContent).toMatch(/游客|Guest/i)
  })

  it('discover header open-settings reaches settings in one tap', async () => {
    render(
      <AppProviders>
        <AnimalPokeApp />
      </AppProviders>,
    )
    fireEvent.click(await screen.findByTestId('open-settings'))
    expect(await screen.findByTestId('settings-screen')).toBeTruthy()
  })

  it('settings opens account panel (bind entry not hidden)', async () => {
    render(
      <AppProviders>
        <AnimalPokeApp />
      </AppProviders>,
    )
    fireEvent.click(await screen.findByTestId('tab-settings'))
    fireEvent.click(await screen.findByTestId('settings-open-account'))
    expect(await screen.findByTestId('account-settings')).toBeTruthy()
  })

  it('read-only shell exposes settings + re-auth + account within two taps', async () => {
    revokeConsent()
    render(
      <AppProviders>
        <AnimalPokeApp />
      </AppProviders>,
    )
    expect(await screen.findByTestId('readonly-shell')).toBeTruthy()
    fireEvent.click(screen.getByTestId('readonly-tab-settings'))
    expect(await screen.findByTestId('settings-screen')).toBeTruthy()
    expect(screen.getByTestId('settings-readonly-banner')).toBeTruthy()
    expect(screen.getByTestId('settings-reauth')).toBeTruthy()
    fireEvent.click(screen.getByTestId('settings-reauth'))
    await waitFor(() => {
      // after re-auth, ConsentGate should leave readonly and mount children
      // (or at least clear readonly banner if still on settings unit path)
      expect(
        screen.queryByTestId('settings-readonly-banner') === null ||
          screen.queryByTestId('readonly-shell') === null,
      ).toBe(true)
    })
  })

  it('read-only shell account tab opens bind panel', async () => {
    revokeConsent()
    render(
      <AppProviders>
        <AnimalPokeApp />
      </AppProviders>,
    )
    fireEvent.click(await screen.findByTestId('readonly-tab-account'))
    expect(await screen.findByTestId('account-settings')).toBeTruthy()
  })

  it('deep link #settings lands on settings screen when consented', async () => {
    location.hash = '#settings'
    render(
      <AppProviders>
        <AnimalPokeApp />
      </AppProviders>,
    )
    expect(await screen.findByTestId('settings-screen')).toBeTruthy()
  })
})
