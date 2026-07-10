/** Outdoor safety pure rules (AP-045) */

import type { WeatherType } from '../weather/types'
import {
  MAX_ACCURACY_M,
  LATE_NIGHT_START_HOUR,
  LATE_NIGHT_END_HOUR,
  LOW_BATTERY_THRESHOLD,
  HIGH_SPEED_MPS,
  DEFAULT_UNSAFE_ZONES,
  type UnsafeZone,
  type UnsafeZoneType,
} from './constants'
import { calculateDistance } from '../lbs/logic'

export type OutdoorPauseReason =
  | 'extreme_weather'
  | 'low_battery'
  | 'late_night'
  | 'high_speed'
  | 'low_accuracy'
  | 'unsafe_zone'

export interface OutdoorSafetyInput {
  weather?: WeatherType | null
  batteryLevel?: number | null
  batteryCharging?: boolean | null
  /** Local hour 0–23; if omitted uses clock */
  hour?: number | null
  speedMps?: number | null
  accuracyM?: number | null
  now?: Date | number
}

export interface OutdoorSafetyResult {
  allowed: boolean
  reasons: OutdoorPauseReason[]
  messages: string[]
}

export interface SpawnCandidate {
  lat: number
  lng: number
}

const REASON_MESSAGES: Record<OutdoorPauseReason, string> = {
  extreme_weather: '极端天气，户外捕获已暂停',
  low_battery: '电量过低，户外捕获已暂停',
  late_night: '深夜时段，请注意安全，户外捕获已暂停',
  high_speed: '移动速度过快，请停下后再操作',
  low_accuracy: '定位精度不足，无法进入捕获范围',
  unsafe_zone: '当前位置不适合生成发现点',
}

/** Local hour from clock (injectable for tests) */
export function getLocalHour(now: Date | number = Date.now()): number {
  const d = typeof now === 'number' ? new Date(now) : now
  return d.getHours()
}

/** True during late-night protection window [23:00, 05:00) */
export function isLateNight(hour: number): boolean {
  return hour >= LATE_NIGHT_START_HOUR || hour < LATE_NIGHT_END_HOUR
}

/** True when battery is critically low and not charging */
export function isLowBattery(
  level: number | null | undefined,
  charging: boolean | null | undefined,
): boolean {
  if (level == null || Number.isNaN(level)) return false
  if (charging === true) return false
  return level < LOW_BATTERY_THRESHOLD
}

/** True when GPS accuracy exceeds threshold */
export function isAccuracyTooLow(accuracyM: number | null | undefined): boolean {
  if (accuracyM == null || Number.isNaN(accuracyM)) return false
  return accuracyM > MAX_ACCURACY_M
}

/** True when moving too fast for safe outdoor capture */
export function isHighSpeed(speedMps: number | null | undefined): boolean {
  if (speedMps == null || Number.isNaN(speedMps) || speedMps < 0) return false
  return speedMps > HIGH_SPEED_MPS
}

/** Extreme weather pauses outdoor capture */
export function isExtremeWeather(weather: WeatherType | null | undefined): boolean {
  return weather === 'extreme'
}

/** Distance check against unsafe zone */
export function isInsideUnsafeZone(
  point: SpawnCandidate,
  zones: UnsafeZone[] = DEFAULT_UNSAFE_ZONES,
): { unsafe: boolean; type?: UnsafeZoneType; zone?: UnsafeZone } {
  for (const zone of zones) {
    const dist = calculateDistance(
      { lat: point.lat, lng: point.lng },
      { lat: zone.lat, lng: zone.lng },
    )
    if (dist <= zone.radiusM) {
      return { unsafe: true, type: zone.type, zone }
    }
  }
  return { unsafe: false }
}

/** Filter spawn candidates that fall on roads/water/construction/private/unreachable */
export function filterSafeSpawnPoints<T extends SpawnCandidate>(
  candidates: T[],
  zones: UnsafeZone[] = DEFAULT_UNSAFE_ZONES,
): T[] {
  return candidates.filter((c) => !isInsideUnsafeZone(c, zones).unsafe)
}

/**
 * Generate a safe spawn position near center by rejection sampling.
 * Falls back to farthest-from-zone offset if all attempts fail.
 */
export function generateSafeSpawnPosition(
  center: SpawnCandidate,
  minMeters: number,
  maxMeters: number,
  zones: UnsafeZone[] = DEFAULT_UNSAFE_ZONES,
  rand: () => number = Math.random,
  maxAttempts = 12,
): SpawnCandidate | null {
  for (let i = 0; i < maxAttempts; i++) {
    const distance = minMeters + rand() * (maxMeters - minMeters)
    const angle = rand() * Math.PI * 2
    const latPerMeter = 1 / 111000
    const lngPerMeter = 1 / (111000 * Math.cos(center.lat * Math.PI / 180))
    const candidate = {
      lat: center.lat + distance * Math.cos(angle) * latPerMeter,
      lng: center.lng + distance * Math.sin(angle) * lngPerMeter,
    }
    if (!isInsideUnsafeZone(candidate, zones).unsafe) {
      return candidate
    }
  }
  return null
}

/** Aggregate outdoor capture eligibility */
export function evaluateOutdoorSafety(input: OutdoorSafetyInput): OutdoorSafetyResult {
  const reasons: OutdoorPauseReason[] = []
  const hour =
    input.hour != null
      ? input.hour
      : getLocalHour(input.now ?? Date.now())

  if (isExtremeWeather(input.weather)) reasons.push('extreme_weather')
  if (isLowBattery(input.batteryLevel, input.batteryCharging)) reasons.push('low_battery')
  if (isLateNight(hour)) reasons.push('late_night')
  if (isHighSpeed(input.speedMps)) reasons.push('high_speed')
  if (isAccuracyTooLow(input.accuracyM)) reasons.push('low_accuracy')

  return {
    allowed: reasons.length === 0,
    reasons,
    messages: reasons.map((r) => REASON_MESSAGES[r]),
  }
}

/**
 * Whether a discovery point can be marked in-range.
 * Requires distance + accuracy; outdoor pause reasons block capture.
 */
export function canMarkInRange(opts: {
  distanceM: number
  captureRangeM: number
  accuracyM?: number | null
  outdoor?: OutdoorSafetyResult
}): boolean {
  if (opts.outdoor && !opts.outdoor.allowed) return false
  if (isAccuracyTooLow(opts.accuracyM)) return false
  return opts.distanceM <= opts.captureRangeM
}

export { REASON_MESSAGES, MAX_ACCURACY_M }
