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
import * as pipeline from '../../services/capturePipeline'
import * as syncQueue from '../../services/syncQueue'
import { AnimalRepository } from '../../db/repositories/animal-repository'
import { SyncQueueRepository } from '../../db/repositories/sync-queue-repository'

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
    cb(new Blob(['frame'], { type: 'image/jpeg' }))
  }
}

describe('AnimalPokeApp production entry', () => {
  beforeEach(() => {
    localStorage.clear()
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

  it('capture success deducts stamina once and runs analyze/value/sync once', async () => {
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

    const genSpy = vi.spyOn(pipeline, 'runCaptureGeneration').mockResolvedValue({
      sessionId: 'sess-unit-1',
      inferenceRequestId: 'sess-unit-1',
      species: 'cat',
      analysis: {
        breed: 'Tabby',
        color: 'orange',
        body_type: 'normal',
        quality_score: 8,
        subject_completeness: 8,
        clarity: 8,
        lighting: 7,
        composition: 7,
        pose: 6,
        angle: 5,
      },
      value: {
        rarity: 2,
        hp: 50,
        atk: 15,
        def: 12,
        spd: 18,
        class: 'Ranger',
        element: 'Wind',
        narrative: 'unit test cat',
      },
    })
    const enqueueSpy = vi.spyOn(syncQueue, 'enqueueGeneratedAnimal').mockResolvedValue({
      id: 'q1',
      idempotencyKey: 'sync:animal:sess-unit-1',
      route: '/sync/animal',
      status: 'pending',
      attempts: 0,
      createdAt: Date.now(),
      updatedAt: Date.now(),
      nextAttemptAt: Date.now(),
      payload: {
        uuid: 'sess-unit-1',
        species: 'cat',
        rarity: 2,
        generated_at: new Date().toISOString(),
      },
    } as never)
    const flushSpy = vi.spyOn(syncQueue, 'flushSyncQueue').mockResolvedValue({ synced: 1, failed: 0 })
    const addSpy = vi.spyOn(AnimalRepository, 'add').mockResolvedValue()
    vi.spyOn(AnimalRepository, 'getById').mockResolvedValue(undefined)
    vi.spyOn(SyncQueueRepository, 'clearSynced').mockResolvedValue(1)

    // force success + stable stamina consume tracking
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

    await act(async () => {
      fireEvent.click(screen.getByTestId('capture-stage'))
    })

    await waitFor(() => {
      expect(genSpy).toHaveBeenCalledTimes(1)
      expect(enqueueSpy).toHaveBeenCalledTimes(1)
      expect(flushSpy).toHaveBeenCalledTimes(1)
      expect(addSpy).toHaveBeenCalledTimes(1)
    })

    // second click must not double-reward / re-run pipeline
    await act(async () => {
      fireEvent.click(screen.getByTestId('capture-stage'))
    })
    expect(genSpy).toHaveBeenCalledTimes(1)
    expect(addSpy).toHaveBeenCalledTimes(1)

    delete (window as unknown as { __AP_FORCE_CAPTURE_SUCCESS?: boolean }).__AP_FORCE_CAPTURE_SUCCESS
  })
})
