/**
 * A/B experiment helper — requires stop condition, sample size, and welfare guardrails.
 * Experiments that omit safety fields are rejected.
 */

export type ExperimentStopCondition = {
  /** Hard stop after this many assignments (including control). */
  maxAssignments?: number
  /** Hard stop after duration from start. */
  maxDurationMs?: number
  /** Minimum conversions before evaluating primary metric. */
  minConversions?: number
  /** Stop if harm / welfare incident rate exceeds this (0–1). */
  maxHarmRate?: number
  /** Primary metric gate: stop when relative lift CI excludes 0 (placeholder flag). */
  requireSignificance?: boolean
}

export type WelfareGuardrails = {
  /** Explicit acknowledgement that experiment does not encourage animal harm. */
  noAnimalHarm: boolean
  /** Disallow mechanics that pressure repeated wildlife disturbance. */
  maxCaptureAttemptsPerSession: number
  /** Human-readable safety note for reviewers. */
  animalSafetyNote: string
  /** Optional: block experiments for minors. */
  excludeMinors?: boolean
  /** Optional: kill-switch metric name that forces stop. */
  killSwitchMetric?: string
}

export type ExperimentDefinition = {
  id: string
  name: string
  variants: readonly string[]
  /** Required minimum planned sample size (total assignments). */
  sampleSize: number
  /** Required stop condition — at least one bound must be set. */
  stopCondition: ExperimentStopCondition
  /** Required animal welfare / safety guardrails. */
  welfareGuardrails: WelfareGuardrails
  startedAt?: number
  assignments?: number
  harmIncidents?: number
}

export type ExperimentValidationError =
  | 'missing_id'
  | 'missing_variants'
  | 'missing_sample_size'
  | 'invalid_sample_size'
  | 'missing_stop_condition'
  | 'empty_stop_condition'
  | 'missing_welfare_guardrails'
  | 'welfare_no_animal_harm_required'
  | 'welfare_max_attempts_required'
  | 'welfare_safety_note_required'

export type ExperimentValidationResult =
  | { ok: true; experiment: ExperimentDefinition }
  | { ok: false; errors: ExperimentValidationError[] }

function hasStopBound(stop: ExperimentStopCondition): boolean {
  return (
    (typeof stop.maxAssignments === 'number' && stop.maxAssignments > 0) ||
    (typeof stop.maxDurationMs === 'number' && stop.maxDurationMs > 0) ||
    (typeof stop.minConversions === 'number' && stop.minConversions > 0) ||
    (typeof stop.maxHarmRate === 'number' && stop.maxHarmRate >= 0) ||
    stop.requireSignificance === true
  )
}

export function validateExperiment(raw: Partial<ExperimentDefinition>): ExperimentValidationResult {
  const errors: ExperimentValidationError[] = []
  if (!raw.id || typeof raw.id !== 'string') errors.push('missing_id')
  if (!raw.variants || raw.variants.length < 2) errors.push('missing_variants')
  if (raw.sampleSize == null) errors.push('missing_sample_size')
  else if (typeof raw.sampleSize !== 'number' || raw.sampleSize < 1) errors.push('invalid_sample_size')
  if (raw.stopCondition == null) errors.push('missing_stop_condition')
  else if (!hasStopBound(raw.stopCondition)) errors.push('empty_stop_condition')
  if (raw.welfareGuardrails == null) errors.push('missing_welfare_guardrails')
  else {
    const w = raw.welfareGuardrails
    if (w.noAnimalHarm !== true) errors.push('welfare_no_animal_harm_required')
    if (typeof w.maxCaptureAttemptsPerSession !== 'number' || w.maxCaptureAttemptsPerSession < 1) {
      errors.push('welfare_max_attempts_required')
    }
    if (!w.animalSafetyNote || !String(w.animalSafetyNote).trim()) {
      errors.push('welfare_safety_note_required')
    }
  }
  if (errors.length) return { ok: false, errors }

  const experiment: ExperimentDefinition = {
    id: raw.id!,
    name: raw.name || raw.id!,
    variants: raw.variants!,
    sampleSize: raw.sampleSize!,
    stopCondition: raw.stopCondition!,
    welfareGuardrails: raw.welfareGuardrails!,
    startedAt: raw.startedAt ?? Date.now(),
    assignments: raw.assignments ?? 0,
    harmIncidents: raw.harmIncidents ?? 0,
  }
  return { ok: true, experiment }
}

/** Create experiment only when validation passes. */
export function defineExperiment(def: Partial<ExperimentDefinition>): ExperimentDefinition {
  const result = validateExperiment(def)
  if (!result.ok) {
    throw new Error(`Invalid experiment: ${result.errors.join(',')}`)
  }
  return result.experiment
}

export type AssignmentResult = {
  variant: string
  stopped: boolean
  reason?: 'sample_size' | 'max_assignments' | 'duration' | 'harm_rate' | 'not_registered'
}

/**
 * Deterministic-ish assignment from session id hash; respects stop conditions.
 */
export function assignVariant(
  experiment: ExperimentDefinition,
  sessionId: string,
  now = Date.now(),
): AssignmentResult {
  if (shouldStopExperiment(experiment, now).stop) {
    const s = shouldStopExperiment(experiment, now)
    return { variant: experiment.variants[0], stopped: true, reason: s.reason }
  }
  const idx = hashToIndex(sessionId + experiment.id, experiment.variants.length)
  return { variant: experiment.variants[idx], stopped: false }
}

export function shouldStopExperiment(
  experiment: ExperimentDefinition,
  now = Date.now(),
): { stop: boolean; reason?: AssignmentResult['reason'] } {
  const sc = experiment.stopCondition
  const assignments = experiment.assignments ?? 0
  if (assignments >= experiment.sampleSize) {
    return { stop: true, reason: 'sample_size' }
  }
  if (typeof sc.maxAssignments === 'number' && assignments >= sc.maxAssignments) {
    return { stop: true, reason: 'max_assignments' }
  }
  if (
    typeof sc.maxDurationMs === 'number' &&
    experiment.startedAt != null &&
    now - experiment.startedAt >= sc.maxDurationMs
  ) {
    return { stop: true, reason: 'duration' }
  }
  if (typeof sc.maxHarmRate === 'number' && assignments > 0) {
    const rate = (experiment.harmIncidents ?? 0) / assignments
    if (rate > sc.maxHarmRate) return { stop: true, reason: 'harm_rate' }
  }
  return { stop: false }
}

function hashToIndex(input: string, modulo: number): number {
  let h = 0
  for (let i = 0; i < input.length; i++) {
    h = (h * 31 + input.charCodeAt(i)) >>> 0
  }
  return modulo === 0 ? 0 : h % modulo
}
