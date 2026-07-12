/**
 * AP-062: capture success reveal only after local save; no double pipeline.
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, cleanup, fireEvent, waitFor, act } from '@testing-library/react'
import CaptureScreen from './CaptureScreen'
import { AppProviders } from '../../../providers/AppProviders'
import { grantConsent } from '../../../compliance'
import * as pipeline from '../../../services/capturePipeline'
import * as syncQueue from '../../../services/syncQueue'
import { AnimalRepository } from '../../../db/repositories/animal-repository'

vi.mock('../../../outdoorSafety/logic', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../../outdoorSafety/logic')>()
  return {
    ...actual,
    evaluateOutdoorSafety: () => ({ allowed: true, messages: [], stopFirst: false }),
  }
})

describe('CaptureScreen post-hit stages (AP-062)', () => {
  beforeEach(() => {
    localStorage.clear()
    sessionStorage.clear()
    localStorage.setItem(
      'animal-poke-onboarding-v1',
      JSON.stringify({ step: 'done', skipped: true, completedAt: Date.now() }),
    )
    grantConsent()
    ;(window as unknown as { __AP_FORCE_CAPTURE_SUCCESS?: boolean }).__AP_FORCE_CAPTURE_SUCCESS = true
  })
  afterEach(() => {
    cleanup()
    vi.restoreAllMocks()
    delete (window as unknown as { __AP_FORCE_CAPTURE_SUCCESS?: boolean }).__AP_FORCE_CAPTURE_SUCCESS
  })

  it('does not reveal 捕获成功 before save; reveals after pipeline save', async () => {
    const gen = vi.spyOn(pipeline, 'runCaptureGeneration').mockImplementation(async () => {
      // delay so we can observe intermediate stage
      await new Promise((r) => setTimeout(r, 30))
      return {
        sessionId: 'sess-ap062',
        inferenceRequestId: 'inf-v',
        valueInferenceId: 'inf-v',
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
          narrative: 'test',
        },
      }
    })
    const addSpy = vi.spyOn(AnimalRepository, 'add').mockResolvedValue()
    vi.spyOn(AnimalRepository, 'getById').mockResolvedValue(undefined)
    vi.spyOn(syncQueue, 'enqueueGeneratedAnimal').mockResolvedValue({} as never)
    vi.spyOn(syncQueue, 'flushSyncQueue').mockResolvedValue({ synced: 1, failed: 0 })

    const onSettled = vi.fn()
    render(
      <AppProviders>
        <CaptureScreen
          onToast={() => {}}
          species="cat"
          detection={{
            species: 'cat',
            confidence: 0.9,
            boundingBox: [0.1, 0.1, 0.3, 0.3],
          }}
          detectInferenceId="inf-d"
          captureAttemptId="attempt-ap062"
          onSettled={onSettled}
        />
      </AppProviders>,
    )

    const stage = screen.getByTestId('capture-stage')
    await act(async () => {
      fireEvent.pointerDown(stage)
      fireEvent.pointerUp(stage)
    })

    // intermediate: must not show success reveal yet while generating
    await waitFor(() => {
      expect(screen.getByTestId('capture-post-hit-stage')).toBeTruthy()
    })
    expect(screen.queryByTestId('capture-success-reveal')).toBeNull()

    await waitFor(() => {
      expect(gen).toHaveBeenCalledTimes(1)
      expect(addSpy).toHaveBeenCalledTimes(1)
      expect(screen.getByTestId('capture-success-reveal')).toBeTruthy()
    })
    expect(onSettled).toHaveBeenCalledWith(true)

    // second throw must not re-run pipeline
    await act(async () => {
      fireEvent.pointerDown(stage)
      fireEvent.pointerUp(stage)
    })
    expect(gen).toHaveBeenCalledTimes(1)
  })
})
