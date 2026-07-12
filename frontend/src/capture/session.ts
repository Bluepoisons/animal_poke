import type { SpeciesType } from '../types'
import type { DetectionResult } from '../services/visionDetect'
import { CAPTURE_STAMINA_COST as SHARED_CAPTURE_STAMINA_COST } from '../stamina/constants'

export type CapturePhase =
  | 'idle'
  | 'armed'
  | 'throwing'
  | 'settling'
  | 'success'
  | 'failed'
  | 'cancelled'

export type CaptureSession = {
  id: string
  idempotencyKey: string
  phase: CapturePhase
  species: SpeciesType
  detection?: DetectionResult
  power: number
  settled: boolean
  staminaSpent: boolean
  startedAt: number
  targetId?: string
}

/** Single source of truth (AP-032): stamina/constants.ts */
export const CAPTURE_STAMINA_COST = SHARED_CAPTURE_STAMINA_COST
const BEST_MIN = 35
const BEST_MAX = 75

export function createCaptureSession(input: {
  species?: SpeciesType
  detection?: DetectionResult
  targetId?: string
  power?: number
}): CaptureSession {
  const id =
    typeof crypto !== 'undefined' && 'randomUUID' in crypto
      ? crypto.randomUUID()
      : `cap-${Date.now()}`
  return {
    id,
    idempotencyKey: `capture-${id}`,
    phase: 'armed',
    species: input.detection?.species || input.species || (() => { throw new Error('species_required') })(),
    detection: input.detection,
    power: input.power ?? 55,
    settled: false,
    staminaSpent: false,
    startedAt: Date.now(),
    targetId: input.targetId,
  }
}

export function successProbability(
  power: number,
  bestMin = BEST_MIN,
  bestMax = BEST_MAX,
): number {
  if (power >= bestMin && power <= bestMax) return 0.78
  const dist = power < bestMin ? bestMin - power : power - bestMax
  return Math.max(0.15, 0.78 - dist * 0.02)
}

export type SettleInput = {
  session: CaptureSession
  online: boolean
  stamina: number
  consumeStamina: (n: number) => boolean
  rng?: () => number
}

export type SettleResult =
  | { ok: true; session: CaptureSession }
  | { ok: false; reason: 'offline' | 'no_stamina' | 'already_settled' | 'failed'; session: CaptureSession }

/** 一次性结算：扣体力 + 概率判定，同一 session 只允许一次 */
export function settleCapture(input: SettleInput): SettleResult {
  const session = { ...input.session }
  if (session.settled) {
    return { ok: false, reason: 'already_settled', session }
  }
  if (!input.online) {
    session.phase = 'failed'
    return { ok: false, reason: 'offline', session }
  }
  if (!session.staminaSpent) {
    if (input.stamina < CAPTURE_STAMINA_COST) {
      session.phase = 'failed'
      return { ok: false, reason: 'no_stamina', session }
    }
    const spent = input.consumeStamina(CAPTURE_STAMINA_COST)
    if (!spent) {
      session.phase = 'failed'
      return { ok: false, reason: 'no_stamina', session }
    }
    session.staminaSpent = true
  }
  const roll = (input.rng || Math.random)()
  const p = successProbability(session.power)
  session.settled = true
  if (roll <= p) {
    session.phase = 'success'
    return { ok: true, session }
  }
  session.phase = 'failed'
  return { ok: false, reason: 'failed', session }
}

export { BEST_MIN, BEST_MAX }
