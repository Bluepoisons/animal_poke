import { describe, it, expect } from 'vitest'
import { createInitialCaptureFlow, reduceCaptureFlow } from './captureFlow'

describe('recognition correction', () => {
  it('allows re-detect after reset', () => {
    let s = createInitialCaptureFlow()
    s = reduceCaptureFlow(s, { type: 'START_DETECT', photoBlob: new Blob(['x']) })
    s = reduceCaptureFlow(s, {
      type: 'DETECT_SUCCESS',
      detectInferenceId: 'inf',
      detections: [{ id: '1', species: 'cat', confidence: 0.9, boundingBox: [0, 0, 0.2, 0.2] }],
    })
    expect(s.selectedBox?.species).toBe('cat')
    s = reduceCaptureFlow(s, { type: 'RESET' })
    expect(s.phase).toBe('idle')
    s = reduceCaptureFlow(s, { type: 'START_DETECT', photoBlob: new Blob(['y']) })
    s = reduceCaptureFlow(s, {
      type: 'DETECT_SUCCESS',
      detectInferenceId: 'inf2',
      detections: [{ id: '2', species: 'dog', confidence: 0.91, boundingBox: [0, 0, 0.2, 0.2] }],
    })
    expect(s.selectedBox?.species).toBe('dog')
  })
})
