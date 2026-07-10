import { describe, it, expect, beforeEach } from 'vitest'
import {
  defaultGameConfig,
  validateGameConfig,
  applyGameConfig,
  getEconomyConfig,
  __resetGameConfigForTests,
  GAME_CONFIG_BOUNDS,
} from './gameConfig'

describe('gameConfig (AP-059)', () => {
  beforeEach(() => {
    __resetGameConfigForTests()
  })

  it('default config is valid and matches code constants', () => {
    const cfg = defaultGameConfig()
    expect(validateGameConfig(cfg)).toEqual([])
    expect(cfg.economy.captureStaminaCost).toBe(20)
    expect(cfg.economy.staminaRecoveryPerHour).toBe(10)
    expect(cfg.economy.toyBallPrice).toBe(50)
  })

  it('rejects illegal values outside hard bounds', () => {
    const cfg = defaultGameConfig()
    cfg.economy.captureStaminaCost = 0
    cfg.economy.toyBallPrice = -1
    const errs = validateGameConfig(cfg)
    expect(errs.some((e) => e.includes('captureStaminaCost'))).toBe(true)
    expect(errs.some((e) => e.includes('toyBallPrice'))).toBe(true)
  })

  it('clamps remote overrides into bounds', () => {
    applyGameConfig({ economy: { captureStaminaCost: 9999, toyBallPrice: 1 } as Partial<import("./gameConfig").GameEconomyConfig> })
    const e = getEconomyConfig()
    expect(e.captureStaminaCost).toBe(GAME_CONFIG_BOUNDS.captureStaminaCost[1])
    expect(e.toyBallPrice).toBe(1)
  })

  it('supports rollback to previous version payload without rebuild', () => {
    applyGameConfig({ version: 'game-config.v1-bad', economy: { captureStaminaCost: 30 } as Partial<import('./gameConfig').GameEconomyConfig> })
    expect(getEconomyConfig().captureStaminaCost).toBe(30)
    applyGameConfig({ version: 'game-config.v1', economy: { captureStaminaCost: 20 } as Partial<import('./gameConfig').GameEconomyConfig> })
    expect(getEconomyConfig().captureStaminaCost).toBe(20)
  })
})
