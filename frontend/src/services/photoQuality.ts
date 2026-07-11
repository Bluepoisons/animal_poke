/**
 * AP-098 Photography quality skill — client metrics + feedback helpers.
 * Server remains authoritative; client only estimates local metrics for UX
 * and submits them for deterministic scoring.
 *
 * Rules:
 * - Never encourage "get closer" for rarity.
 * - Safe distance is a first-class skill dimension.
 * - Every daily theme has an accessibility alternative.
 */

export const PHOTO_QUALITY_CONFIG_VERSION = 'photo-quality-v1'

export type PhotoBand = 'excellent' | 'good' | 'fair' | 'poor'

export type PhotoA11yMode = 'timer_hold' | 'high_contrast_frame' | 'voice_guided' | 'static_upload'

export interface PhotoMetrics {
  stability_rms: number
  subject_fill_ratio: number
  subject_center_offset: number
  lighting_score: number
  occlusion_ratio: number
  subject_completeness: number
  estimated_distance_m?: number
  sensor_samples: number
  device_model?: string
}

export interface PhotoDimensionScores {
  stability: number
  subject_completeness: number
  lighting: number
  occlusion: number
  composition: number
  safe_distance: number
}

export interface PhotoScoreResult {
  overall: number
  band: PhotoBand
  dimensions: PhotoDimensionScores
  tips: string[]
  welfare_flags?: string[]
  research_bonus: number
  chase_penalty: boolean
  config_version: string
  metrics_digest: string
  signature?: string
  rarity_eligible: boolean
}

export interface PhotoA11yGoal {
  goal_id: string
  title: string
  description: string
  mode: PhotoA11yMode
}

export interface PhotoDailyTheme {
  date: string
  theme_id: string
  title: string
  description: string
  target_dimension: string
  target_score: number
  accessibility_alternative: PhotoA11yGoal
  config_version: string
}

export interface PhotoCalibration {
  baseline_stability_rms: number
  lighting_offset: number
  sample_count: number
  calibrated: boolean
  config_version: string
}

const FILL_SWEET_MIN = 0.12
const FILL_SWEET_MAX = 0.55
const FILL_TOO_CLOSE = 0.72

function clamp01(v: number): number {
  if (!Number.isFinite(v)) return 0
  if (v < 0) return 0
  if (v > 1) return 1
  return v
}

function round3(v: number): number {
  return Math.round(v * 1000) / 1000
}

/** Local preview scoring (mirrors server rules for offline UX). Not authoritative. */
export function previewPhotoScore(
  m: PhotoMetrics,
  cal: PhotoCalibration = defaultCalibration(),
): Omit<PhotoScoreResult, 'metrics_digest' | 'signature' | 'config_version'> & {
  config_version: string
  metrics_digest: string
} {
  const baseline = cal.baseline_stability_rms > 0 ? cal.baseline_stability_rms : 0.08
  let rms = m.stability_rms
  if (m.sensor_samples > 0 && m.sensor_samples < 3) {
    rms = Math.max(rms, baseline * 1.5)
  }
  let stability = clamp01(baseline / Math.max(rms, 1e-6))
  if (m.sensor_samples === 0 && rms === 0) stability = 0.45

  const completeness = clamp01(m.subject_completeness)
  const lighting = clamp01(m.lighting_score + (cal.lighting_offset || 0))
  const occlusionClear = clamp01(1 - m.occlusion_ratio)

  const centerScore = clamp01(1 - m.subject_center_offset / 0.5)
  const fill = m.subject_fill_ratio
  let fillScore = 0
  if (fill < FILL_SWEET_MIN) fillScore = clamp01(fill / FILL_SWEET_MIN)
  else if (fill <= FILL_SWEET_MAX) fillScore = 1
  else if (fill < FILL_TOO_CLOSE) {
    fillScore = clamp01(1 - ((fill - FILL_SWEET_MAX) / (FILL_TOO_CLOSE - FILL_SWEET_MAX)) * 0.5)
  } else fillScore = 0.25
  const composition = clamp01(centerScore * 0.55 + fillScore * 0.45)

  const safe = safeDistancePreview(fill, m.estimated_distance_m ?? 0)
  const dimensions: PhotoDimensionScores = {
    stability: round3(stability),
    subject_completeness: round3(completeness),
    lighting: round3(lighting),
    occlusion: round3(occlusionClear),
    composition: round3(composition),
    safe_distance: round3(safe),
  }
  const overall = round3(
    clamp01(
      dimensions.stability * 0.18 +
        dimensions.subject_completeness * 0.18 +
        dimensions.lighting * 0.16 +
        dimensions.occlusion * 0.14 +
        dimensions.composition * 0.16 +
        dimensions.safe_distance * 0.18,
    ),
  )
  const chase = fill >= FILL_TOO_CLOSE || ((m.estimated_distance_m ?? 0) > 0 && (m.estimated_distance_m ?? 0) < 1.5)
  const welfare_flags: string[] = []
  if (chase) welfare_flags.push('too_close_do_not_chase')
  if (m.occlusion_ratio >= 0.7) welfare_flags.push('heavy_occlusion')
  if (dimensions.lighting < 0.3) welfare_flags.push('poor_lighting')

  return {
    overall,
    band: bandOf(overall),
    dimensions,
    tips: buildTips(dimensions, chase),
    welfare_flags,
    research_bonus: chase ? round3(overall * 0.5) : overall,
    chase_penalty: chase,
    rarity_eligible: !chase && overall >= 0.25,
    config_version: PHOTO_QUALITY_CONFIG_VERSION,
    metrics_digest: `local:${Math.round(overall * 1000)}`,
  }
}

function safeDistancePreview(fill: number, distanceM: number): number {
  let fromFill = 0.4
  if (fill <= 0) fromFill = 0.4
  else if (fill < 0.08) fromFill = 0.55
  else if (fill <= 0.45) fromFill = 0.7 + 0.3 * clamp01((fill - 0.08) / 0.2)
  else if (fill < FILL_TOO_CLOSE) fromFill = clamp01(1 - (fill - 0.45) / (FILL_TOO_CLOSE - 0.45))
  else fromFill = 0.05

  if (distanceM <= 0) return clamp01(fromFill)
  let fromDist = 0.55
  if (distanceM < 1.5) fromDist = 0.05
  else if (distanceM < 3) fromDist = 0.4 + 0.4 * ((distanceM - 1.5) / 1.5)
  else if (distanceM <= 12) fromDist = 1
  else if (distanceM <= 40) fromDist = 0.7
  return clamp01(fromFill * 0.55 + fromDist * 0.45)
}

function bandOf(q: number): PhotoBand {
  const s = q * 10
  if (s >= 8) return 'excellent'
  if (s >= 5) return 'good'
  if (s >= 3) return 'fair'
  return 'poor'
}

function buildTips(d: PhotoDimensionScores, chase: boolean): string[] {
  const cands: { score: number; tip: string }[] = [
    { score: d.stability, tip: 'Hold steadier for a few seconds before capturing' },
    { score: d.subject_completeness, tip: 'Frame the whole animal when safe to do so' },
    { score: d.lighting, tip: 'Seek even natural light; avoid harsh backlight' },
    { score: d.occlusion, tip: 'Move sideways for a clearer line of sight — do not approach' },
    { score: d.composition, tip: 'Center the subject with comfortable framing' },
    { score: d.safe_distance, tip: 'Keep a respectful distance; closer never raises rarity' },
  ]
  cands.sort((a, b) => a.score - b.score)
  const out: string[] = []
  for (const c of cands) {
    if (c.score >= 0.75) continue
    out.push(c.tip)
    if (out.length >= 3) break
  }
  if (chase) {
    out.unshift('Too close — step back. Do not chase or crowd animals.')
    return out.slice(0, 3)
  }
  return out.length ? out : ['Great observation — keep practicing safe, steady framing']
}

export function defaultCalibration(): PhotoCalibration {
  return {
    baseline_stability_rms: 0.08,
    lighting_offset: 0,
    sample_count: 0,
    calibrated: false,
    config_version: PHOTO_QUALITY_CONFIG_VERSION,
  }
}

/** Estimate fill ratio from a detection box {x,y,w,h} in normalized 0-1 coords. */
export function fillRatioFromBox(box?: { w?: number; h?: number; width?: number; height?: number } | null): number {
  if (!box) return 0
  const w = Number(box.w ?? box.width ?? 0)
  const h = Number(box.h ?? box.height ?? 0)
  if (!Number.isFinite(w) || !Number.isFinite(h) || w <= 0 || h <= 0) return 0
  return clamp01(w * h)
}

export function centerOffsetFromBox(box?: { x?: number; y?: number; w?: number; h?: number } | null): number {
  if (!box) return 0.5
  const x = Number(box.x ?? 0)
  const y = Number(box.y ?? 0)
  const w = Number(box.w ?? 0)
  const h = Number(box.h ?? 0)
  const cx = x + w / 2
  const cy = y + h / 2
  const dx = cx - 0.5
  const dy = cy - 0.5
  return clamp01(Math.hypot(dx, dy) / 0.7071)
}

/** Explain rarity rule in UI copy — never "get closer". */
export function rarityQualityHint(score: Pick<PhotoScoreResult, 'chase_penalty' | 'rarity_eligible' | 'overall'>): string {
  if (score.chase_penalty) {
    return 'Too close does not raise rarity. Step back for a safer, higher skill score.'
  }
  if (!score.rarity_eligible) {
    return 'Improve framing and stability — proximity is not a rarity lever.'
  }
  return 'Skillful, respectful observation can support research value and fair quality bands.'
}

/** Dimension label for a11y / UI. */
export const PHOTO_DIMENSION_LABELS: Record<keyof PhotoDimensionScores, string> = {
  stability: 'Stability',
  subject_completeness: 'Subject completeness',
  lighting: 'Lighting',
  occlusion: 'Clear view',
  composition: 'Composition',
  safe_distance: 'Safe distance',
}
