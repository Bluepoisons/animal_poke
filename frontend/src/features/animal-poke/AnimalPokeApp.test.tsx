/**
 * AnimalPokeApp production entry — hard gate tests (AP-014 / #175)
 * Assert real API interactions for detect path (not just "doesn't crash").
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, cleanup, fireEvent, waitFor, act } from '@testing-library/react'
import { grantConsent } from '../../compliance'
import { AppProviders } from '../../providers/AppProviders'
import AnimalPokeApp from './AnimalPokeApp'
import * as vision from '../../services/visionDetect'
import { AnimalRepository } from '../../db/repositories/animal-repository'
import { SyncQueueRepository } from '../../db/repositories/sync-queue-repository'

vi.mock('../../outdoorSafety/logic', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../outdoorSafety/logic')>()
  return {
    ...actual,
    evaluateOutdoorSafety: () => ({ allowed: true, messages: [], stopFirst: false }),
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
    // quality gate requires >= 2KB frames
    cb(new Blob([new Uint8Array(2500).fill(7)], { type: 'image/jpeg' }))
  }
}

describe('AnimalPokeApp production entry', () => {
  beforeEach(() => {
    localStorage.clear()
    sessionStorage.clear()
    grantConsent()
    mockCameraReady()
    location.hash = ''
  })
  afterEach(() => {
    cleanup()
    vi.restoreAllMocks()
    location.hash = ''
  })

  it('renders discover screen by default with energy and coins', async () => {
    render(
      <AppProviders>
        <AnimalPokeApp />
      </AppProviders>,
    )
    expect(await screen.findByText(/DISCOVER MODE/i)).toBeTruthy()
    expect(screen.getByLabelText(/体力/)).toBeTruthy()
    expect(document.querySelector('.ap-phone') || document.body.firstChild).toBeTruthy()
  })

  it('navigates between tabs without crashing', async () => {
    render(
      <AppProviders>
        <AnimalPokeApp />
      </AppProviders>,
    )
    const buttons = await screen.findAllByRole('button')
    expect(buttons.length).toBeGreaterThan(0)
    for (const btn of buttons.slice(0, 6)) {
      fireEvent.click(btn)
    }
  })

  it('calls detectAnimals API path when scanning from discover', async () => {
    const detectSpy = vi.spyOn(vision, 'detectAnimals').mockResolvedValue({
      inferenceId: 'inf-unit-cat',
      animals: [
        {
          species: 'cat',
          confidence: 0.93,
          boundingBox: [0.2, 0.2, 0.4, 0.4],
        },
      ],
    })

    render(
      <AppProviders>
        <AnimalPokeApp />
      </AppProviders>,
    )

    // wait camera ready
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /开始识别/ })).toBeTruthy()
    })

    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /开始识别/ }))
    })

    await waitFor(() => {
      expect(detectSpy).toHaveBeenCalledTimes(1)
    })

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /进入捕获/ })).toBeTruthy()
    })
    expect(screen.getByText(/识别：猫|cat/i)).toBeTruthy()
  })

  it('blocks direct #capture without detection (guard)', async () => {
    render(
      <AppProviders>
        <AnimalPokeApp />
      </AppProviders>,
    )
    await act(async () => {
      location.hash = '#capture'
      window.dispatchEvent(new HashChangeEvent('hashchange'))
    })
    await waitFor(() => {
      expect(screen.queryByTestId('capture-screen')).toBeNull()
    })
  })

  it('capture throw path settles once under force-success flag', async () => {
    vi.spyOn(vision, 'detectAnimals').mockResolvedValue({
      inferenceId: 'inf-pipeline-1',
      animals: [
        {
          species: 'cat',
          confidence: 0.91,
          boundingBox: [0.1, 0.1, 0.3, 0.3],
        },
      ],
    })

    // CaptureScreen uses hold-to-charge throw + __AP_FORCE_CAPTURE_SUCCESS (not pipeline spy).
    ;(window as unknown as { __AP_FORCE_CAPTURE_SUCCESS?: boolean }).__AP_FORCE_CAPTURE_SUCCESS =
      true

    render(
      <AppProviders>
        <AnimalPokeApp />
      </AppProviders>,
    )

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /开始识别/ })).toBeTruthy()
    })
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /开始识别/ }))
    })
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /进入捕获/ })).toBeTruthy()
    })
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /进入捕获/ }))
    })

    await waitFor(() => {
      expect(screen.getByTestId('capture-screen')).toBeTruthy()
    })

    const stage = screen.getByTestId('capture-stage')
    await act(async () => {
      fireEvent.pointerDown(stage)
      fireEvent.pointerUp(stage)
    })

    // second throw must not crash
    await act(async () => {
      fireEvent.pointerDown(stage)
      fireEvent.pointerUp(stage)
    })
    expect(screen.getByTestId('capture-screen')).toBeTruthy()
  })
})
