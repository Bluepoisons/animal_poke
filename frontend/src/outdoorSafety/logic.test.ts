import { describe, it, expect } from 'vitest'
import {
  isLateNight,
  isLowBattery,
  isAccuracyTooLow,
  isHighSpeed,
  isExtremeWeather,
  isInsideUnsafeZone,
  filterSafeSpawnPoints,
  generateSafeSpawnPosition,
  evaluateOutdoorSafety,
  canMarkInRange,
} from './logic'
import { DEFAULT_UNSAFE_ZONES, MAX_ACCURACY_M, HIGH_SPEED_MPS } from './constants'

describe('isLateNight', () => {
  it('23:00 is late night', () => {
    expect(isLateNight(23)).toBe(true)
  })
  it('0–4 are late night', () => {
    expect(isLateNight(0)).toBe(true)
    expect(isLateNight(4)).toBe(true)
  })
  it('5:00 and daytime are not late night', () => {
    expect(isLateNight(5)).toBe(false)
    expect(isLateNight(12)).toBe(false)
    expect(isLateNight(22)).toBe(false)
  })
})

describe('isLowBattery', () => {
  it('blocks when below 15% and not charging', () => {
    expect(isLowBattery(0.1, false)).toBe(true)
    expect(isLowBattery(0.14, null)).toBe(true)
  })
  it('allows when charging or level ok', () => {
    expect(isLowBattery(0.1, true)).toBe(false)
    expect(isLowBattery(0.5, false)).toBe(false)
    expect(isLowBattery(null, false)).toBe(false)
  })
})

describe('isAccuracyTooLow', () => {
  it(`blocks when accuracy > ${MAX_ACCURACY_M}m`, () => {
    expect(isAccuracyTooLow(MAX_ACCURACY_M + 1)).toBe(true)
    expect(isAccuracyTooLow(MAX_ACCURACY_M)).toBe(false)
    expect(isAccuracyTooLow(undefined)).toBe(false)
  })
})

describe('isHighSpeed', () => {
  it(`blocks when speed > ${HIGH_SPEED_MPS} m/s`, () => {
    expect(isHighSpeed(HIGH_SPEED_MPS + 1)).toBe(true)
    expect(isHighSpeed(5)).toBe(false)
    expect(isHighSpeed(-1)).toBe(false)
  })
})

describe('isExtremeWeather', () => {
  it('only extreme pauses', () => {
    expect(isExtremeWeather('extreme')).toBe(true)
    expect(isExtremeWeather('rainy')).toBe(false)
    expect(isExtremeWeather(null)).toBe(false)
  })
})

describe('unsafe zone heuristics', () => {
  it('detects road center as unsafe', () => {
    const road = DEFAULT_UNSAFE_ZONES.find((z) => z.type === 'road')!
    const hit = isInsideUnsafeZone({ lat: road.lat, lng: road.lng })
    expect(hit.unsafe).toBe(true)
    expect(hit.type).toBe('road')
  })

  it('detects water, construction, private, unreachable', () => {
    for (const type of ['water', 'construction', 'private', 'unreachable'] as const) {
      const z = DEFAULT_UNSAFE_ZONES.find((x) => x.type === type)!
      expect(isInsideUnsafeZone({ lat: z.lat, lng: z.lng }).type).toBe(type)
    }
  })

  it('filters unsafe candidates', () => {
    const road = DEFAULT_UNSAFE_ZONES.find((z) => z.type === 'road')!
    const safe = { lat: 29.875, lng: 121.555 }
    const result = filterSafeSpawnPoints([
      { lat: road.lat, lng: road.lng },
      safe,
    ])
    expect(result).toHaveLength(1)
    expect(result[0]).toEqual(safe)
  })

  it('generateSafeSpawnPosition never lands on known zones', () => {
    let i = 0
    const seq = [0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 0.15, 0.25, 0.35]
    const rand = () => seq[i++ % seq.length]
    const center = { lat: 29.87, lng: 121.55 }
    const pos = generateSafeSpawnPosition(center, 50, 200, DEFAULT_UNSAFE_ZONES, rand)
    if (pos) {
      expect(isInsideUnsafeZone(pos).unsafe).toBe(false)
    }
  })
})

describe('evaluateOutdoorSafety', () => {
  it('allows safe daytime walk', () => {
    const r = evaluateOutdoorSafety({
      weather: 'sunny',
      batteryLevel: 0.8,
      batteryCharging: false,
      hour: 14,
      speedMps: 1.2,
      accuracyM: 12,
    })
    expect(r.allowed).toBe(true)
    expect(r.reasons).toEqual([])
  })

  it('aggregates multiple pause reasons', () => {
    const r = evaluateOutdoorSafety({
      weather: 'extreme',
      batteryLevel: 0.05,
      batteryCharging: false,
      hour: 1,
      speedMps: 20,
      accuracyM: 80,
    })
    expect(r.allowed).toBe(false)
    expect(r.reasons).toContain('extreme_weather')
    expect(r.reasons).toContain('low_battery')
    expect(r.reasons).toContain('late_night')
    expect(r.reasons).toContain('high_speed')
    expect(r.reasons).toContain('low_accuracy')
    expect(r.messages.length).toBe(r.reasons.length)
  })
})

describe('canMarkInRange', () => {
  it('requires distance, accuracy, and outdoor allow', () => {
    expect(
      canMarkInRange({ distanceM: 20, captureRangeM: 50, accuracyM: 10 }),
    ).toBe(true)
    expect(
      canMarkInRange({ distanceM: 20, captureRangeM: 50, accuracyM: 80 }),
    ).toBe(false)
    expect(
      canMarkInRange({
        distanceM: 20,
        captureRangeM: 50,
        accuracyM: 10,
        outdoor: { allowed: false, reasons: ['high_speed'], messages: ['x'] },
      }),
    ).toBe(false)
    expect(
      canMarkInRange({ distanceM: 60, captureRangeM: 50, accuracyM: 10 }),
    ).toBe(false)
  })
})
