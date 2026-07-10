/** Economy model parameters derived from game constants (AP-051) */

import { CAPTURE_STAMINA_COST, DISPATCH_STAMINA_COST, POTION_PRICE, POTION_RECOVERY, STAMINA_RECOVERY_PER_HOUR } from '../../stamina/constants'
import { BATTLE_GOLD_REWARDS, BATTLE_STAMINA_COST, BATTLE_LOSE_GOLD, BATTLE_DRAW_GOLD } from '../../battle/constants'
import { CHECK_IN_REWARDS, ITEM_DEFS } from '../../shop/constants'
import { DISPATCH_MISSION_MAP, DISPATCH_SPEEDUP_COST } from '../constants'

/** Capture gold by rarity (aligned with battle multipliers scale) */
export const CAPTURE_GOLD: Record<string, number> = {
  common: 12,
  uncommon: 20,
  rare: 35,
  epic: 60,
  legendary: 100,
}

/** Duplicate capture gold fraction */
export const DUP_CAPTURE_GOLD_FACTOR = 0.35

export const ECONOMY_MODEL = {
  captureStaminaCost: CAPTURE_STAMINA_COST,
  dispatchStaminaCost: DISPATCH_STAMINA_COST,
  battleStaminaCost: BATTLE_STAMINA_COST,
  potionPrice: POTION_PRICE,
  potionRecovery: POTION_RECOVERY,
  staminaRecoveryPerHour: STAMINA_RECOVERY_PER_HOUR,
  checkInRewards: CHECK_IN_REWARDS,
  battleGold: BATTLE_GOLD_REWARDS,
  battleLoseGold: BATTLE_LOSE_GOLD,
  battleDrawGold: BATTLE_DRAW_GOLD,
  captureGold: CAPTURE_GOLD,
  dupCaptureFactor: DUP_CAPTURE_GOLD_FACTOR,
  dispatchBaseGold: {
    quick: DISPATCH_MISSION_MAP.quick.baseGold,
    standard: DISPATCH_MISSION_MAP.standard.baseGold,
    deep: DISPATCH_MISSION_MAP.deep.baseGold,
  },
  dispatchSpeedupCost: DISPATCH_SPEEDUP_COST,
  toyBallPrice: ITEM_DEFS.toy_ball.price,
  maxStaminaStart: 120,
  startingGold: 100,
} as const

export type PlayerArchetype = 'newbie' | 'casual' | 'core' | 'returning'

export interface ArchetypeProfile {
  id: PlayerArchetype
  /** Captures attempted per active day */
  capturesPerDay: number
  /** Battles per day */
  battlesPerDay: number
  /** Dispatch missions per day */
  dispatchesPerDay: number
  /** Probability of daily check-in */
  checkInRate: number
  /** Probability a capture is a duplicate */
  dupRate: number
  /** Shop spend attempts per day (buy toy ball / potion when gold allows) */
  shopBuysPerDay: number
  /** Active day fraction over horizon (returning players skip days) */
  activeDayRate: number
  /** Win rate in battle */
  battleWinRate: number
}

export const ARCHETYPES: Record<PlayerArchetype, ArchetypeProfile> = {
  newbie: {
    id: 'newbie',
    capturesPerDay: 3,
    battlesPerDay: 1,
    dispatchesPerDay: 1,
    checkInRate: 0.9,
    dupRate: 0.15,
    shopBuysPerDay: 0.5,
    activeDayRate: 1,
    battleWinRate: 0.45,
  },
  casual: {
    id: 'casual',
    capturesPerDay: 2,
    battlesPerDay: 1,
    dispatchesPerDay: 1,
    checkInRate: 0.7,
    dupRate: 0.35,
    shopBuysPerDay: 0.3,
    activeDayRate: 0.85,
    battleWinRate: 0.5,
  },
  core: {
    id: 'core',
    capturesPerDay: 6,
    battlesPerDay: 3,
    dispatchesPerDay: 2,
    checkInRate: 0.95,
    dupRate: 0.55,
    shopBuysPerDay: 1.2,
    activeDayRate: 1,
    battleWinRate: 0.6,
  },
  returning: {
    id: 'returning',
    capturesPerDay: 4,
    battlesPerDay: 2,
    dispatchesPerDay: 1,
    checkInRate: 0.5,
    dupRate: 0.4,
    shopBuysPerDay: 0.4,
    activeDayRate: 0.4,
    battleWinRate: 0.5,
  },
}
