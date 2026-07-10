import { describe, it, expect } from 'vitest'
import { canEnterCapture, createInitialCaptureFlow, reduceCaptureFlow } from './captureFlow'

describe('navigation capture guard helpers', () => {
  it('blocks capture without confirmed target', () => {
    const s = createInitialCaptureFlow()
    expect(canEnterCapture(s)).toBe(false)
  })

  it('allows capture after single eligible detect', () => {
    let s = createInitialCaptureFlow()
    s = reduceCaptureFlow(s, { type: 'START_DETECT', photoBlob: new Blob(['x']) })
    s = reduceCaptureFlow(s, {
      type: 'DETECT_SUCCESS',
      detectInferenceId: 'inf',
      detections: [
        { id: '1', species: 'cat', confidence: 0.9, boundingBox: [0, 0, 0.2, 0.2] },
      ],
    })
    expect(canEnterCapture(s)).toBe(true)
  })
})
