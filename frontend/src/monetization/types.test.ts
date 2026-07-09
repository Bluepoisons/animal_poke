import { describe, it, expect } from 'vitest'
import {
  CONSUMABLES,
  MONTHLY_CARD,
  getMonthlyCardValue,
  purchase,
  createMockSeason,
  getBattlePassProgress,
} from './types'

describe('monetization', () => {
  it('has consumable products', () => {
    expect(CONSUMABLES.length).toBeGreaterThan(0)
    expect(CONSUMABLES[0].type).toBe('consumable')
  })

  it('monthly card gives better value than direct purchase', () => {
    const value = getMonthlyCardValue(MONTHLY_CARD)
    expect(value).toBeGreaterThan(1) // 1800/300 = 6x value
    expect(value).toBe(6)
  })

  it('purchase succeeds with enough diamonds', () => {
    const result = purchase('gold_pack_s', 100)
    expect(result.success).toBe(true)
    expect(result.newBalance).toBe(40)
  })

  it('purchase fails with insufficient diamonds', () => {
    const result = purchase('gold_pack_l', 100)
    expect(result.success).toBe(false)
    expect(result.error).toBe('钻石不足')
    expect(result.newBalance).toBe(100)
  })

  it('purchase fails for non-existent product', () => {
    const result = purchase('nonexistent', 9999)
    expect(result.success).toBe(false)
    expect(result.error).toBe('产品不存在')
  })

  it('creates valid battle pass season', () => {
    const season = createMockSeason()
    expect(season.maxLevel).toBe(50)
    expect(season.freeTrack.length).toBe(50)
    expect(season.premiumTrack.length).toBe(50)
    expect(season.endDate).toBeGreaterThan(season.startDate)
  })

  it('calculates battle pass progress correctly', () => {
    const season = createMockSeason()
    const progress = getBattlePassProgress(season, 2500)
    expect(progress.level).toBe(3)
    expect(progress.expIntoLevel).toBe(500)
    expect(progress.expForNextLevel).toBe(1000)
  })

  it('caps battle pass at max level', () => {
    const season = createMockSeason()
    const progress = getBattlePassProgress(season, 999999)
    expect(progress.level).toBe(season.maxLevel)
  })
})
