/** Outdoor safety constants (AP-045) */

/** Max GPS accuracy (meters) allowed for in-range capture */
export const MAX_ACCURACY_M = 50

/** Night window (local hour, inclusive start exclusive end for late night) */
export const LATE_NIGHT_START_HOUR = 23
export const LATE_NIGHT_END_HOUR = 5

/** Battery level below which outdoor capture pauses (0–1) */
export const LOW_BATTERY_THRESHOLD = 0.15

/** Speed (m/s) above which capture is blocked (~vehicle, ~54 km/h) */
export const HIGH_SPEED_MPS = 15

/** Weather types that pause outdoor capture */
export const EXTREME_WEATHER_TYPES = ['extreme'] as const

/** Zone types that must not host spawn points */
export type UnsafeZoneType =
  | 'road'
  | 'water'
  | 'construction'
  | 'private'
  | 'unreachable'

/** Heuristic unsafe zones relative to a city grid (demo / server-side rules) */
export interface UnsafeZone {
  type: UnsafeZoneType
  /** Center lat */
  lat: number
  /** Center lng */
  lng: number
  /** Exclusion radius in meters */
  radiusM: number
  label?: string
}

/**
 * Built-in heuristic zones around Ningbo demo coords.
 * Production would load from map provider; these are deterministic rules for tests.
 */
export const DEFAULT_UNSAFE_ZONES: UnsafeZone[] = [
  // Road corridor
  { type: 'road', lat: 29.8705, lng: 121.5505, radiusM: 25, label: 'main_road' },
  // Water body
  { type: 'water', lat: 29.8680, lng: 121.5480, radiusM: 80, label: 'river' },
  // Construction
  { type: 'construction', lat: 29.8720, lng: 121.5520, radiusM: 40, label: 'site_a' },
  // Private property
  { type: 'private', lat: 29.8710, lng: 121.5490, radiusM: 30, label: 'compound' },
  // Unreachable (steep bank)
  { type: 'unreachable', lat: 29.8690, lng: 121.5530, radiusM: 35, label: 'cliff' },
]
