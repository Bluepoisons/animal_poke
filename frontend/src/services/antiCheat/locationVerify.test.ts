import { describe, it, expect } from 'vitest'
import { analyzeLocation } from './locationVerify'

describe('locationVerify', () => {
  it('should not flag normal GPS location', () => {
    const result = analyzeLocation({
      lat: 29.8729,
      lng: 121.5420,
      accuracy: 15,
      timestamp: Date.now(),
    })
    expect(result.mockDetected).toBe(false)
    expect(result.mockSignals).toHaveLength(0)
  })

  it('should flag accuracy too high (mock GPS)', () => {
    const result = analyzeLocation({
      lat: 29.8729,
      lng: 121.5420,
      accuracy: 1,
      timestamp: Date.now(),
    })
    expect(result.mockDetected).toBe(true)
    expect(result.mockSignals[0]).toContain('accuracy_too_high')
  })

  it('should flag speed anomaly (teleport)', () => {
    const now = Date.now()
    const result = analyzeLocation(
      { lat: 39.9042, lng: 116.4074, accuracy: 10, timestamp: now + 30000 }, // Beijing
      { lat: 31.2304, lng: 121.4737, accuracy: 10, timestamp: now }, // Shanghai ~1200km in 30s
    )
    expect(result.mockDetected).toBe(true)
    expect(result.mockSignals.some(s => s.includes('speed_anomaly'))).toBe(true)
  })

  it('should not flag normal walking speed', () => {
    const now = Date.now()
    const result = analyzeLocation(
      { lat: 29.8730, lng: 121.5421, accuracy: 10, timestamp: now + 10000 },
      { lat: 29.8729, lng: 121.5420, accuracy: 10, timestamp: now },
    )
    expect(result.mockDetected).toBe(false)
  })

  it('should flag null island (0,0)', () => {
    const result = analyzeLocation({
      lat: 0,
      lng: 0,
      accuracy: 10,
      timestamp: Date.now(),
    })
    expect(result.mockDetected).toBe(true)
    expect(result.mockSignals.some(s => s.includes('null_island'))).toBe(true)
  })
})
