import { useMemo } from 'react'
import type { WeatherType } from '../weather/types'
import {
  evaluateOutdoorSafety,
  type OutdoorSafetyResult,
  type OutdoorSafetyInput,
} from './logic'
import { MAX_ACCURACY_M } from './constants'

export interface UseOutdoorSafetyOptions {
  weather?: WeatherType | null
  speedMps?: number | null
  accuracyM?: number | null
  batteryLevel?: number | null
  batteryCharging?: boolean | null
  hour?: number | null
}

/**
 * Evaluate outdoor capture safety from available signals.
 * Battery uses Navigator.getBattery when available (async snapshotted by caller).
 */
export function useOutdoorSafety(opts: UseOutdoorSafetyOptions): OutdoorSafetyResult {
  return useMemo(() => {
    const input: OutdoorSafetyInput = {
      weather: opts.weather,
      speedMps: opts.speedMps,
      accuracyM: opts.accuracyM,
      batteryLevel: opts.batteryLevel,
      batteryCharging: opts.batteryCharging,
      hour: opts.hour,
    }
    return evaluateOutdoorSafety(input)
  }, [
    opts.weather,
    opts.speedMps,
    opts.accuracyM,
    opts.batteryLevel,
    opts.batteryCharging,
    opts.hour,
  ])
}

/** Accuracy circle radius for map UI (meters, clamped) */
export function accuracyCircleRadiusM(accuracyM: number | null | undefined): number {
  if (accuracyM == null || Number.isNaN(accuracyM) || accuracyM <= 0) return 0
  return Math.min(accuracyM, MAX_ACCURACY_M * 4)
}

export { evaluateOutdoorSafety, MAX_ACCURACY_M }
