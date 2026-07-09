import { describe, it, expect } from 'vitest'
import { isVirtualCamera, analyzeMotionVariance } from './cameraVerify'
import type { MotionSample } from './types'

describe('cameraVerify', () => {
  it('should detect virtual camera labels', () => {
    expect(isVirtualCamera('OBS Virtual Camera')).toBe(true)
    expect(isVirtualCamera('Dummy Video Device')).toBe(true)
    expect(isVirtualCamera('Screen Capture')).toBe(true)
  })

  it('should not flag real camera labels', () => {
    expect(isVirtualCamera('Back Camera')).toBe(false)
    expect(isVirtualCamera('camera2 1, facing back')).toBe(false)
  })

  it('should flag static samples (likely screen capture)', () => {
    const samples: MotionSample[] = Array.from({ length: 10 }, (_, i) => ({
      timestamp: i * 200,
      accelX: 0,
      accelY: 9.8,
      accelZ: 0,
      rotationAlpha: 0,
      rotationBeta: 0,
      rotationGamma: 0,
    }))
    const result = analyzeMotionVariance(samples)
    expect(result.isStatic).toBe(true)
    expect(result.variance).toBeLessThan(0.01)
  })

  it('should detect hand-held motion', () => {
    const samples: MotionSample[] = Array.from({ length: 10 }, (_, i) => ({
      timestamp: i * 200,
      accelX: Math.sin(i) * 0.5,
      accelY: 9.8 + Math.cos(i) * 0.3,
      accelZ: Math.sin(i * 2) * 0.2,
      rotationAlpha: i * 0.1,
      rotationBeta: i * 0.05,
      rotationGamma: 0,
    }))
    const result = analyzeMotionVariance(samples)
    expect(result.isStatic).toBe(false)
    expect(result.variance).toBeGreaterThan(0.01)
  })

  it('should handle insufficient samples', () => {
    const result = analyzeMotionVariance([])
    expect(result.isStatic).toBe(true)
    expect(result.variance).toBe(0)
  })
})
