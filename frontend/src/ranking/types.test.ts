import { describe, it, expect } from 'vitest'
import {
  generateMockLeaderboard,
  isResetTime,
  getCurrentSeason,
  RANK_REWARDS,
} from './types'

describe('ranking', () => {
  it('generates mock leaderboard with sorted entries', () => {
    const result = generateMockLeaderboard('total')
    expect(result.entries.length).toBeGreaterThan(0)
    for (let i = 1; i < result.entries.length; i++) {
      expect(result.entries[i - 1].totalScore).toBeGreaterThanOrEqual(result.entries[i].totalScore)
    }
  })

  it('assigns correct ranks', () => {
    const result = generateMockLeaderboard('total')
    result.entries.forEach((e, i) => {
      expect(e.rank).toBe(i + 1)
    })
  })

  it('sorts perCapita by avgScore', () => {
    const result = generateMockLeaderboard('perCapita')
    for (let i = 1; i < result.entries.length; i++) {
      expect(result.entries[i - 1].avgScore).toBeGreaterThanOrEqual(result.entries[i].avgScore)
    }
  })

  it('sorts progress by progressDelta', () => {
    const result = generateMockLeaderboard('progress')
    for (let i = 1; i < result.entries.length; i++) {
      expect(result.entries[i - 1].progressDelta).toBeGreaterThanOrEqual(result.entries[i].progressDelta)
    }
  })

  it('includes myRegionRank', () => {
    const result = generateMockLeaderboard('total')
    expect(result.myRegionRank).not.toBeNull()
    expect(result.myRegionRank!).toBeGreaterThan(0)
  })

  it('has correct reward tiers', () => {
    expect(RANK_REWARDS.length).toBe(5)
    expect(RANK_REWARDS[0].diamonds).toBe(500)
    expect(RANK_REWARDS[4].diamonds).toBe(50)
  })

  it('detects reset time correctly', () => {
    const midnight = new Date('2026-07-09T00:02:00')
    expect(isResetTime(midnight)).toBe(true)
    const noon = new Date('2026-07-09T12:00:00')
    expect(isResetTime(noon)).toBe(false)
  })

  it('generates season identifier', () => {
    const summer = new Date('2026-07-09T12:00:00')
    const season = getCurrentSeason(summer)
    expect(season).toBe('S2026-07')
  })
})
