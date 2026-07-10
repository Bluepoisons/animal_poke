/**
 * Versioned game configuration (AP-059).
 * Single source for stamina/economy defaults + feature flags.
 * Remote config (window / GET /api/v1/config/game) may override within hard bounds.
 */

import { CAPTURE_STAMINA_COST, DISPATCH_STAMINA_COST, STAMINA_RECOVERY_PER_HOUR, POTION_PRICE, POTION_RECOVERY } from '../stamina/constants'
import { ITEM_DEFS } from '../shop/constants'
import { BATTLE_STAMINA_COST } from '../battle/constants'
import { FEATURE_FLAGS, type FeatureFlagState } from '../features/animal-poke/featureFlags'

export const GAME_CONFIG_VERSION = 'game-config.v1'

export type GameEconomyConfig = {
  captureStaminaCost: number
  dispatchStaminaCost: number
  battleStaminaCost: number
  staminaRecoveryPerHour: number
  potionPrice: number
  potionRecovery: number
  toyBallPrice: number
  premiumToyBallPrice: number
}

export type GameConfig = {
  version: string
  economy: GameEconomyConfig
  features: FeatureFlagState
}

export type GameConfigBounds = {
  captureStaminaCost: [number, number]
  dispatchStaminaCost: [number, number]
  battleStaminaCost: [number, number]
  staminaRecoveryPerHour: [number, number]
  potionPrice: [number, number]
  potionRecovery: [number, number]
  toyBallPrice: [number, number]
  premiumToyBallPrice: [number, number]
}

/** Hard bounds — illegal live-ops values are rejected. */
export const GAME_CONFIG_BOUNDS: GameConfigBounds = {
  captureStaminaCost: [1, 120],
  dispatchStaminaCost: [1, 120],
  battleStaminaCost: [1, 120],
  staminaRecoveryPerHour: [1, 60],
  potionPrice: [1, 10_000],
  potionRecovery: [1, 100],
  toyBallPrice: [1, 10_000],
  premiumToyBallPrice: [1, 10_000],
}

export function defaultGameConfig(): GameConfig {
  return {
    version: GAME_CONFIG_VERSION,
    economy: {
      captureStaminaCost: CAPTURE_STAMINA_COST,
      dispatchStaminaCost: DISPATCH_STAMINA_COST,
      battleStaminaCost: BATTLE_STAMINA_COST,
      staminaRecoveryPerHour: STAMINA_RECOVERY_PER_HOUR,
      potionPrice: POTION_PRICE,
      potionRecovery: POTION_RECOVERY,
      toyBallPrice: ITEM_DEFS.toy_ball.price,
      premiumToyBallPrice: ITEM_DEFS.premium_toy_ball.price,
    },
    features: { ...FEATURE_FLAGS },
  }
}

export function validateGameConfig(cfg: GameConfig): string[] {
  const errors: string[] = []
  if (!cfg?.version) errors.push('version required')
  const e = cfg.economy
  if (!e) {
    errors.push('economy required')
    return errors
  }
  for (const [key, [min, max]] of Object.entries(GAME_CONFIG_BOUNDS) as [keyof GameEconomyConfig, [number, number]][]) {
    const v = e[key]
    if (typeof v !== 'number' || Number.isNaN(v) || v < min || v > max) {
      errors.push(`${key} out of bounds [${min},${max}]: ${v}`)
    }
  }
  return errors
}

function clampEconomy(partial: Partial<GameEconomyConfig>, base: GameEconomyConfig): GameEconomyConfig {
  const out = { ...base }
  for (const key of Object.keys(GAME_CONFIG_BOUNDS) as (keyof GameEconomyConfig)[]) {
    if (partial[key] === undefined) continue
    const [min, max] = GAME_CONFIG_BOUNDS[key]
    const n = Number(partial[key])
    if (Number.isFinite(n)) out[key] = Math.min(max, Math.max(min, n))
  }
  return out
}

let active: GameConfig = defaultGameConfig()

export function getGameConfig(): GameConfig {
  return active
}

export function getEconomyConfig(): GameEconomyConfig {
  return active.economy
}

/**
 * Merge remote/runtime config over defaults. Invalid fields are clamped or ignored.
 * Returns validation errors (empty if fully valid before clamp).
 */
export function applyGameConfig(partial: Partial<Omit<GameConfig, "economy">> & { economy?: Partial<GameEconomyConfig> }): string[] {
  const base = defaultGameConfig()
  const next: GameConfig = {
    version: partial.version || base.version,
    economy: clampEconomy(partial.economy || {}, base.economy),
    features: { ...base.features, ...(partial.features || {}) },
  }
  const errors = validateGameConfig(next)
  // After clamp, re-validate should pass; if not, keep previous active
  const post = validateGameConfig(next)
  if (post.length === 0) {
    active = next
  }
  return errors
}

/** Load from window.__AP_CONFIG__.game if present (runtime inject / rollback without rebuild). */
export function loadGameConfigFromWindow(): void {
  if (typeof window === 'undefined') return
  const g = (window as unknown as { __AP_CONFIG__?: { game?: Partial<GameConfig> } }).__AP_CONFIG__?.game
  if (g) applyGameConfig(g)
}

/** Test helper */
export function __resetGameConfigForTests(): void {
  active = defaultGameConfig()
}
