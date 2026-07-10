/**
 * Funnel metrics helpers — success/unknown rates, stage drop-off, latency buckets.
 */

import {
  FUNNEL_STAGES,
  type AnalyticsEventBase,
  type FunnelEventName,
} from './schema'

export type RateResult = {
  total: number
  success: number
  unknown: number
  error: number
  successRate: number
  unknownRate: number
}

export type StageDropOff = {
  stage: FunnelEventName
  count: number
  dropOffFromPrev: number | null
  retentionFromStart: number
}

export type PercentileResult = {
  p50: number | null
  p95: number | null
  n: number
}

/** Detect success / unknown rates from detect_result events. */
export function computeDetectRates(events: AnalyticsEventBase[]): RateResult {
  const detects = events.filter((e) => e.name === 'detect_result')
  let success = 0
  let unknown = 0
  let error = 0
  for (const e of detects) {
    const outcome = String(e.props?.outcome ?? '')
    if (outcome === 'success') success++
    else if (outcome === 'unknown') unknown++
    else error++
  }
  const total = detects.length
  return {
    total,
    success,
    unknown,
    error,
    successRate: total === 0 ? 0 : success / total,
    unknownRate: total === 0 ? 0 : unknown / total,
  }
}

/** Stage drop-off along the core funnel using unique session_ids per stage. */
export function computeStageDropOff(events: AnalyticsEventBase[]): StageDropOff[] {
  const sessionsByStage = new Map<FunnelEventName, Set<string>>()
  for (const stage of FUNNEL_STAGES) {
    sessionsByStage.set(stage, new Set())
  }
  for (const e of events) {
    if (!sessionsByStage.has(e.name as FunnelEventName)) continue
    sessionsByStage.get(e.name as FunnelEventName)!.add(e.session_id)
  }

  const startCount = sessionsByStage.get(FUNNEL_STAGES[0])?.size ?? 0
  const out: StageDropOff[] = []
  let prev = 0
  FUNNEL_STAGES.forEach((stage, idx) => {
    const count = sessionsByStage.get(stage)?.size ?? 0
    const dropOffFromPrev = idx === 0 || prev === 0 ? null : 1 - count / prev
    const retentionFromStart = startCount === 0 ? 0 : count / startCount
    out.push({ stage, count, dropOffFromPrev, retentionFromStart })
    prev = count
  })
  return out
}

/** Simple percentile on numeric samples (e.g. latency ms). */
export function computePercentiles(samples: number[]): PercentileResult {
  if (samples.length === 0) return { p50: null, p95: null, n: 0 }
  const sorted = [...samples].sort((a, b) => a - b)
  const at = (p: number) => {
    const idx = Math.min(sorted.length - 1, Math.max(0, Math.ceil(p * sorted.length) - 1))
    return sorted[idx]
  }
  return { p50: at(0.5), p95: at(0.95), n: sorted.length }
}

/** Capture attempts per successful collection (calls-per-capture). */
export function computeCaptureCallRate(events: AnalyticsEventBase[]): {
  attempts: number
  completes: number
  attemptsPerCapture: number | null
} {
  const attempts = events.filter((e) => e.name === 'capture_attempt').length
  const completes = events.filter(
    (e) => e.name === 'collection_complete' && e.props?.result === 'saved',
  ).length
  return {
    attempts,
    completes,
    attemptsPerCapture: completes === 0 ? null : attempts / completes,
  }
}

/** Count repeat clicks: same name + session within window. */
export function countRepeatClicks(
  events: AnalyticsEventBase[],
  name: FunnelEventName,
  windowMs = 2000,
): number {
  const filtered = events
    .filter((e) => e.name === name)
    .sort((a, b) => a.ts - b.ts)
  let repeats = 0
  for (let i = 1; i < filtered.length; i++) {
    if (
      filtered[i].session_id === filtered[i - 1].session_id &&
      filtered[i].ts - filtered[i - 1].ts <= windowMs
    ) {
      repeats++
    }
  }
  return repeats
}

/**
 * D1 / D7 retention from session start markers.
 * Expects events with name auth (or any) carrying day-bucketed activity;
 * callers pass distinct active days keyed by pseudo session cohort day0.
 */
export function computeRetention(
  day0Sessions: Set<string>,
  activeByDayOffset: Map<number, Set<string>>,
): { d1: number; d7: number; cohortSize: number } {
  const cohortSize = day0Sessions.size
  if (cohortSize === 0) return { d1: 0, d7: 0, cohortSize: 0 }
  const d1set = activeByDayOffset.get(1) ?? new Set()
  const d7set = activeByDayOffset.get(7) ?? new Set()
  let d1 = 0
  let d7 = 0
  for (const s of day0Sessions) {
    if (d1set.has(s)) d1++
    if (d7set.has(s)) d7++
  }
  return { d1: d1 / cohortSize, d7: d7 / cohortSize, cohortSize }
}
