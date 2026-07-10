import {
  CONTINUOUS_SCAN_MS,
  CONTINUOUS_SCAN_MS_SAVER,
  FRAME_BUDGET_MS,
  JANK_RATIO_THRESHOLD,
  LOW_BATTERY_LEVEL,
  UPLOAD_MAX_EDGE_DEFAULT,
  UPLOAD_MAX_EDGE_LOW,
  UPLOAD_MAX_EDGE_SAVER,
  UPLOAD_QUALITY_DEFAULT,
  UPLOAD_QUALITY_LOW,
  UPLOAD_QUALITY_SAVER,
} from './constants'
import type {
  ImageCompressOptions,
  PerfDecision,
  PerfSignals,
  PerfTier,
  ScanMode,
} from './types'

/** Slow network heuristic */
export function isSlowNetwork(signals: PerfSignals['network']): boolean {
  if (!signals.online) return true
  if (signals.saveData === true) return true
  const t = (signals.effectiveType || '').toLowerCase()
  if (t === 'slow-2g' || t === '2g' || t === '3g') return true
  if (signals.downlinkMbps != null && signals.downlinkMbps > 0 && signals.downlinkMbps < 1.5) {
    return true
  }
  return false
}

export function isLowBattery(battery: PerfSignals['battery']): boolean {
  if (battery.level == null || Number.isNaN(battery.level)) return false
  if (battery.charging === true) return false
  return battery.level < LOW_BATTERY_LEVEL
}

export function isJanky(jankRatio: number | null | undefined): boolean {
  if (jankRatio == null || Number.isNaN(jankRatio)) return false
  return jankRatio >= JANK_RATIO_THRESHOLD
}

/** Decide perf tier + scan mode from signals (pure) */
export function decidePerfMode(signals: PerfSignals): PerfDecision {
  const reasons: string[] = []
  let tier: PerfTier = 'full'

  if (signals.userDataSaver || signals.network.saveData) {
    tier = 'saver'
    reasons.push('data_saver')
  }
  if (isSlowNetwork(signals.network)) {
    tier = tier === 'full' ? 'saver' : tier
    reasons.push('slow_network')
  }
  if (isLowBattery(signals.battery)) {
    tier = 'low'
    reasons.push('low_battery')
  }
  if (isJanky(signals.jankRatio)) {
    tier = 'low'
    reasons.push('jank')
  }
  if (!signals.network.online) {
    tier = tier === 'full' ? 'saver' : tier
    reasons.push('offline')
  }

  let scanMode: ScanMode = 'continuous'
  if (tier !== 'full') scanMode = 'manual'
  // Continuous only when online + full tier
  if (!signals.network.online) scanMode = 'manual'

  const uploadMaxEdge =
    tier === 'low'
      ? UPLOAD_MAX_EDGE_LOW
      : tier === 'saver'
        ? UPLOAD_MAX_EDGE_SAVER
        : UPLOAD_MAX_EDGE_DEFAULT
  const uploadQuality =
    tier === 'low'
      ? UPLOAD_QUALITY_LOW
      : tier === 'saver'
        ? UPLOAD_QUALITY_SAVER
        : UPLOAD_QUALITY_DEFAULT
  const continuousScanMs = tier === 'full' ? CONTINUOUS_SCAN_MS : CONTINUOUS_SCAN_MS_SAVER

  return {
    tier,
    scanMode,
    uploadMaxEdge,
    uploadQuality,
    continuousScanMs,
    pauseCameraOnBackground: true,
    reasons,
  }
}

/** Update jank ratio from frame durations */
export function updateJankRatio(
  samples: number[],
  frameMs: number,
  maxSamples: number,
  budgetMs: number = FRAME_BUDGET_MS,
): { samples: number[]; ratio: number } {
  const next = [...samples, frameMs > budgetMs ? 1 : 0]
  while (next.length > maxSamples) next.shift()
  const ratio = next.length === 0 ? 0 : next.reduce((a, b) => a + b, 0) / next.length
  return { samples: next, ratio }
}

/**
 * Scale + compress image blob before upload.
 * Uses createImageBitmap + canvas when available; otherwise returns original.
 */
export async function compressImageForUpload(
  blob: Blob,
  opts: ImageCompressOptions,
): Promise<Blob> {
  if (typeof createImageBitmap !== 'function' || typeof document === 'undefined') {
    return blob
  }
  try {
    const bitmap = await createImageBitmap(blob)
    const maxEdge = Math.max(1, opts.maxEdge)
    const scale = Math.min(1, maxEdge / Math.max(bitmap.width, bitmap.height))
    const w = Math.max(1, Math.round(bitmap.width * scale))
    const h = Math.max(1, Math.round(bitmap.height * scale))
    const canvas = document.createElement('canvas')
    canvas.width = w
    canvas.height = h
    const ctx = canvas.getContext('2d')
    if (!ctx) {
      bitmap.close()
      return blob
    }
    ctx.drawImage(bitmap, 0, 0, w, h)
    bitmap.close()
    const mime = opts.mimeType ?? 'image/jpeg'
    const quality = opts.quality
    const out = await new Promise<Blob | null>((resolve) => {
      canvas.toBlob((b) => resolve(b), mime, quality)
    })
    return out ?? blob
  } catch {
    return blob
  }
}

/** Virtual list window: which items to mount given scroll */
export function virtualWindow(
  total: number,
  scrollTop: number,
  viewportHeight: number,
  itemHeight: number,
  overscan = 4,
): { start: number; end: number; offsetY: number } {
  if (total <= 0 || itemHeight <= 0) return { start: 0, end: 0, offsetY: 0 }
  const start = Math.max(0, Math.floor(scrollTop / itemHeight) - overscan)
  const visible = Math.ceil(viewportHeight / itemHeight) + overscan * 2
  const end = Math.min(total, start + visible)
  return { start, end, offsetY: start * itemHeight }
}

/** Thumbnail URL/path helper — prefers small asset when data saver */
export function pickThumbnailSrc(
  full: string,
  thumb: string | undefined,
  dataSaver: boolean,
): string {
  if (dataSaver && thumb) return thumb
  return thumb && dataSaver === false ? full : (thumb ?? full)
}
