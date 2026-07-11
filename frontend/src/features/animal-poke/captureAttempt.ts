/**
 * 捕获 attempt 状态（#178 / AP-017）
 * 一个 encounter 可有多次 attempt；每次独立扣费与结算
 */
import type { SpeciesType } from '../../types'
import { BEST_MAX, BEST_MIN, successProbability } from '../../capture/session'
import { capturableSpeciesIds, getChargeSpeed, getSpeciesDef } from '../../species'

export type AttemptPhase = 'ready' | 'charging' | 'thrown' | 'settled_success' | 'settled_fail' | 'locked'

export interface SpeciesThrowProfile {
  species: SpeciesType
  /** 蓄力速度倍率 */
  chargeSpeed: number
  /** 最佳区间 [min,max] 0-100 */
  bestMin: number
  bestMax: number
  label: string
}

function buildThrowProfile(id: string): SpeciesThrowProfile {
  const def = getSpeciesDef(id)
  const [lo, hi] = def.optimalRange
  return {
    species: id,
    chargeSpeed: getChargeSpeed(id),
    bestMin: lo ?? BEST_MIN,
    bestMax: hi ?? BEST_MAX,
    label: def.captureMechanics || '标准',
  }
}

/** 投掷手感：由内容包 optimal_range / charge_speed 投影 */
export const SPECIES_THROW_PROFILES: Record<string, SpeciesThrowProfile> = Object.fromEntries(
  capturableSpeciesIds().map((id) => [id, buildThrowProfile(id)]),
)

export interface CaptureAttempt {
  id: string
  index: number
  phase: AttemptPhase
  power: number
  settled: boolean
  staminaSpent: boolean
}

export interface EncounterState {
  encounterId: string
  species: SpeciesType
  maxAttempts: number
  attempts: CaptureAttempt[]
  locked: boolean
  success: boolean
}

export function createEncounter(species: SpeciesType, maxAttempts = 3): EncounterState {
  const encounterId =
    typeof crypto !== 'undefined' && 'randomUUID' in crypto
      ? crypto.randomUUID()
      : `enc-${Date.now()}`
  return {
    encounterId,
    species,
    maxAttempts,
    attempts: [createAttempt(0)],
    locked: false,
    success: false,
  }
}

export function createAttempt(index: number): CaptureAttempt {
  const id =
    typeof crypto !== 'undefined' && 'randomUUID' in crypto
      ? crypto.randomUUID()
      : `att-${Date.now()}-${index}`
  return {
    id,
    index,
    phase: 'ready',
    power: 0,
    settled: false,
    staminaSpent: false,
  }
}

export function currentAttempt(enc: EncounterState): CaptureAttempt | null {
  if (enc.locked) return null
  return enc.attempts[enc.attempts.length - 1] ?? null
}

export function canRetry(enc: EncounterState): boolean {
  if (enc.success || enc.locked) return false
  const last = currentAttempt(enc)
  if (!last || last.phase !== 'settled_fail') return false
  return enc.attempts.length < enc.maxAttempts
}

export function beginCharge(enc: EncounterState): EncounterState {
  const att = currentAttempt(enc)
  if (!att || att.settled || enc.locked) return enc
  const attempts = enc.attempts.map((a) =>
    a.id === att.id ? { ...a, phase: 'charging' as const, power: 0 } : a,
  )
  return { ...enc, attempts }
}

export function updatePower(enc: EncounterState, power: number): EncounterState {
  const att = currentAttempt(enc)
  if (!att || att.phase !== 'charging') return enc
  const p = Math.min(100, Math.max(0, Math.round(power)))
  const attempts = enc.attempts.map((a) => (a.id === att.id ? { ...a, power: p } : a))
  return { ...enc, attempts }
}

export function markThrown(enc: EncounterState): EncounterState {
  const att = currentAttempt(enc)
  if (!att || att.settled) return enc
  const attempts = enc.attempts.map((a) =>
    a.id === att.id ? { ...a, phase: 'thrown' as const } : a,
  )
  return { ...enc, attempts }
}

export function settleAttempt(
  enc: EncounterState,
  opts: {
    online: boolean
    stamina: number
    consumeStamina: (n: number) => boolean
    staminaCost?: number
    rng?: () => number
  },
): { enc: EncounterState; ok: boolean; reason?: string } {
  const att = currentAttempt(enc)
  if (!att || att.settled) {
    return { enc, ok: false, reason: 'already_settled' }
  }
  if (!opts.online) {
    const attempts = enc.attempts.map((a) =>
      a.id === att.id ? { ...a, phase: 'settled_fail' as const, settled: true } : a,
    )
    return { enc: { ...enc, attempts }, ok: false, reason: 'offline' }
  }
  const cost = opts.staminaCost ?? 20
  let staminaSpent = att.staminaSpent
  if (!staminaSpent) {
    if (opts.stamina < cost) {
      return { enc, ok: false, reason: 'no_stamina' }
    }
    if (!opts.consumeStamina(cost)) {
      return { enc, ok: false, reason: 'no_stamina' }
    }
    staminaSpent = true
  }
  const profile = SPECIES_THROW_PROFILES[enc.species] ?? buildThrowProfile(enc.species)
  // 使用物种区间微调概率：落在 best 内用标准曲线
  const p = successProbability(att.power)
  const roll = (opts.rng || Math.random)()
  const ok = roll <= p
  const attempts = enc.attempts.map((a) =>
    a.id === att.id
      ? {
          ...a,
          settled: true,
          staminaSpent,
          phase: (ok ? 'settled_success' : 'settled_fail') as AttemptPhase,
        }
      : a,
  )
  if (ok) {
    return {
      enc: { ...enc, attempts, success: true, locked: true },
      ok: true,
    }
  }
  const locked = attempts.length >= enc.maxAttempts
  return {
    enc: { ...enc, attempts, locked },
    ok: false,
    reason: locked ? 'max_attempts' : 'failed',
  }
}

export function startNextAttempt(enc: EncounterState): EncounterState {
  if (!canRetry(enc)) return enc
  return {
    ...enc,
    attempts: [...enc.attempts, createAttempt(enc.attempts.length)],
  }
}

export { BEST_MIN, BEST_MAX, successProbability }
