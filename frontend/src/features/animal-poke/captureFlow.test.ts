import { describe, it, expect } from 'vitest'
import {
  createInitialCaptureFlow,
  reduceCaptureFlow,
  canEnterCapture,
  filterEligibleDetections,
  type DetectedAnimal,
} from './captureFlow'

function animal(partial: Partial<DetectedAnimal> & Pick<DetectedAnimal, 'id' | 'species' | 'confidence'>): DetectedAnimal {
  return {
    ...partial,
  }
}

describe('captureFlow state machine', () => {
  it('CAMERA_READY does not clobber target_confirmed', () => {
    let s = createInitialCaptureFlow()
    s = reduceCaptureFlow(s, { type: 'START_DETECT', photoBlob: new Blob(['x']) })
    s = reduceCaptureFlow(s, {
      type: 'DETECT_SUCCESS',
      detectInferenceId: 'inf-keep',
      detections: [animal({ id: 'a1', species: 'cat', confidence: 0.92 })],
    })
    expect(s.phase).toBe('target_confirmed')
    s = reduceCaptureFlow(s, { type: 'CAMERA_READY' })
    expect(s.phase).toBe('target_confirmed')
    expect(s.selectedBox?.species).toBe('cat')
    s = reduceCaptureFlow(s, { type: 'CAMERA_ERROR', code: 'camera_busy' })
    expect(s.phase).toBe('target_confirmed')
  })

  it('starts idle and becomes camera_ready', () => {
    let s = createInitialCaptureFlow()
    expect(s.phase).toBe('idle')
    s = reduceCaptureFlow(s, { type: 'CAMERA_READY' })
    expect(s.phase).toBe('camera_ready')
  })

  it('auto-confirms single eligible detection', () => {
    let s = createInitialCaptureFlow()
    const blob = new Blob(['x'], { type: 'image/jpeg' })
    s = reduceCaptureFlow(s, { type: 'START_DETECT', photoBlob: blob })
    expect(s.phase).toBe('detecting')
    s = reduceCaptureFlow(s, {
      type: 'DETECT_SUCCESS',
      detectInferenceId: 'inf-1',
      detections: [animal({ id: 'a1', targetId: 'server-target-7', species: 'cat', confidence: 0.92 })],
    })
    expect(s.phase).toBe('target_confirmed')
    expect(s.selectedBox?.species).toBe('cat')
    expect(s.targetId).toBe('server-target-7')
    expect(canEnterCapture(s)).toBe(true)
  })

  it('atomically replaces the species and detect credential after server-confirmed correction', () => {
    let s = createInitialCaptureFlow()
    s = reduceCaptureFlow(s, { type: 'START_DETECT', photoBlob: new Blob(['x']) })
    s = reduceCaptureFlow(s, {
      type: 'DETECT_SUCCESS',
      detectInferenceId: 'inf-original',
      detections: [animal({ id: 'a1', targetId: '0', species: 'cat', label: '猫', confidence: 0.92 })],
    })
    s = reduceCaptureFlow(s, {
      type: 'CORRECTION_SUCCESS',
      detectInferenceId: 'inf-derived',
      animal: animal({ id: 'a1', targetId: '0', species: 'other_animal', label: '赤狐', confidence: 0.92 }),
    })
    expect(s.phase).toBe('target_confirmed')
    expect(s.detectInferenceId).toBe('inf-derived')
    expect(s.selectedBox).toMatchObject({ species: 'other_animal', label: '赤狐', targetId: '0' })
    expect(s.targetId).toBe('0')
    expect(canEnterCapture(s)).toBe(true)
  })

  it('rejects low confidence and unsupported', () => {
    const low = filterEligibleDetections([
      animal({ id: '1', species: 'cat', confidence: 0.5 }),
      animal({ id: '2', species: 'dog', confidence: 0.9 }),
      animal({ id: '3', species: 'whale', confidence: 0.9 }),
      animal({ id: '4', species: 'dragon', confidence: 1 }),
    ])
    expect(low.map((d) => d.species)).toEqual(['dog', 'whale'])
  })

  it('requires selection when multiple animals', () => {
    let s = createInitialCaptureFlow()
    s = reduceCaptureFlow(s, { type: 'START_DETECT', photoBlob: new Blob(['x']) })
    s = reduceCaptureFlow(s, {
      type: 'DETECT_SUCCESS',
      detectInferenceId: 'inf-2',
      detections: [
        animal({ id: 'a', species: 'cat', confidence: 0.9 }),
        animal({ id: 'b', species: 'dog', confidence: 0.9 }),
      ],
    })
    expect(s.errorCode).toBe('need_select_target')
    expect(canEnterCapture(s)).toBe(false)
    s = reduceCaptureFlow(s, { type: 'SELECT_TARGET', animalId: 'b' })
    s = reduceCaptureFlow(s, { type: 'CONFIRM_TARGET' })
    expect(s.phase).toBe('target_confirmed')
    expect(s.selectedBox?.species).toBe('dog')
    expect(canEnterCapture(s)).toBe(true)
  })

  it('fails when no eligible animals', () => {
    let s = createInitialCaptureFlow()
    s = reduceCaptureFlow(s, { type: 'START_DETECT', photoBlob: new Blob(['x']) })
    s = reduceCaptureFlow(s, {
      type: 'DETECT_SUCCESS',
      detectInferenceId: 'inf-3',
      detections: [animal({ id: 'z', species: 'cat', confidence: 0.1 })],
    })
    expect(s.phase).toBe('failed')
    expect(s.errorCode).toBe('no_eligible_animal')
    expect(canEnterCapture(s)).toBe(false)
  })

  it('reset clears photo and detection', () => {
    let s = createInitialCaptureFlow()
    s = reduceCaptureFlow(s, { type: 'START_DETECT', photoBlob: new Blob(['x']) })
    s = reduceCaptureFlow(s, { type: 'RESET' })
    expect(s.phase).toBe('idle')
    expect(s.photoBlob).toBeNull()
    expect(s.detectInferenceId).toBeNull()
  })
})
