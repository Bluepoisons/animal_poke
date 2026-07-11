import { describe, expect, it } from 'vitest'
import {
  centerOffsetFromBox,
  defaultCalibration,
  fillRatioFromBox,
  previewPhotoScore,
  rarityQualityHint,
  type PhotoMetrics,
} from './photoQuality'

function goodMetrics(over: Partial<PhotoMetrics> = {}): PhotoMetrics {
  return {
    stability_rms: 0.06,
    subject_fill_ratio: 0.28,
    subject_center_offset: 0.08,
    lighting_score: 0.72,
    occlusion_ratio: 0.1,
    subject_completeness: 0.85,
    estimated_distance_m: 6,
    sensor_samples: 12,
    ...over,
  }
}

describe('photoQuality (AP-098)', () => {
  it('preview is deterministic for same metrics', () => {
    const a = previewPhotoScore(goodMetrics())
    const b = previewPhotoScore(goodMetrics())
    expect(a.overall).toBe(b.overall)
    expect(a.band).toBe(b.band)
    expect(a.dimensions).toEqual(b.dimensions)
  })

  it('forbids get-closer / chase for rarity', () => {
    const safe = previewPhotoScore(goodMetrics({ subject_fill_ratio: 0.3, estimated_distance_m: 7 }))
    const close = previewPhotoScore(goodMetrics({ subject_fill_ratio: 0.9, estimated_distance_m: 0.8 }))
    expect(safe.rarity_eligible).toBe(true)
    expect(close.rarity_eligible).toBe(false)
    expect(close.chase_penalty).toBe(true)
    expect(close.dimensions.safe_distance).toBeLessThan(safe.dimensions.safe_distance)
    expect(rarityQualityHint(close)).toMatch(/not raise rarity|Step back/i)
  })

  it('device calibration normalizes stability', () => {
    const noisy = defaultCalibration()
    noisy.baseline_stability_rms = 0.2
    noisy.calibrated = true
    const quiet = defaultCalibration()
    quiet.baseline_stability_rms = 0.05
    quiet.calibrated = true
    const rNoisy = previewPhotoScore(goodMetrics({ stability_rms: 0.24 }), noisy)
    const rQuiet = previewPhotoScore(goodMetrics({ stability_rms: 0.06 }), quiet)
    expect(Math.abs(rNoisy.dimensions.stability - rQuiet.dimensions.stability)).toBeLessThan(0.2)
  })

  it('sparse sensors cannot farm perfect stability', () => {
    const r = previewPhotoScore(goodMetrics({ stability_rms: 0.001, sensor_samples: 1 }))
    expect(r.dimensions.stability).toBeLessThan(0.95)
  })

  it('box helpers estimate fill and center', () => {
    expect(fillRatioFromBox({ w: 0.4, h: 0.5 })).toBeCloseTo(0.2, 5)
    expect(centerOffsetFromBox({ x: 0.3, y: 0.3, w: 0.4, h: 0.4 })).toBeLessThan(0.2)
  })

  it('tips explain weak dimensions without closer advice as positive', () => {
    const r = previewPhotoScore(
      goodMetrics({ stability_rms: 1.2, lighting_score: 0.1, subject_fill_ratio: 0.95, estimated_distance_m: 0.5 }),
    )
    expect(r.tips.length).toBeGreaterThan(0)
    // Must not encourage approaching animals
    expect(r.tips.some((t) => /get closer|move closer|come closer/i.test(t))).toBe(false)
    expect(r.tips[0]).toMatch(/Too close|Step back|distance/i)
  })
})
