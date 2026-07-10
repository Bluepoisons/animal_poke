export type ScanMode = 'continuous' | 'manual'

export type PerfTier = 'full' | 'saver' | 'low'

export interface NetworkSignals {
  online: boolean
  /** navigator.connection.effectiveType if available */
  effectiveType?: string | null
  /** Save-Data client hint */
  saveData?: boolean | null
  downlinkMbps?: number | null
}

export interface BatterySignals {
  level?: number | null
  charging?: boolean | null
}

export interface PerfSignals {
  network: NetworkSignals
  battery: BatterySignals
  /** Rolling jank ratio 0–1 */
  jankRatio?: number | null
  /** User forced data saver from settings */
  userDataSaver?: boolean
}

export interface PerfDecision {
  tier: PerfTier
  scanMode: ScanMode
  uploadMaxEdge: number
  uploadQuality: number
  continuousScanMs: number
  pauseCameraOnBackground: boolean
  reasons: string[]
}

export interface ImageCompressOptions {
  maxEdge: number
  quality: number
  mimeType?: string
}
