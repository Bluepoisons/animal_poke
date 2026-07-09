import { describe, it, expect } from 'vitest'
import {
  getTierByRating,
  calculateRatingDelta,
  applyMatchResult,
  getMatchRange,
  isInMatchRange,
  getWinRate,
  TIER_THRESHOLDS,
} from './types'

const basePlayer = {
  playerId: 'p1',
  wins: 10,
  losses: 5,
  winStreak: 0,
}

describe('pvp', () => {
  it('assigns correct tier by rating', () => {
    expect(getTierByRating(500)).toBe('bronze')
    expect(getTierByRating(1200)).toBe('silver')
    expect(getTierByRating(1500)).toBe('gold')
    expect(getTierByRating(1800)).toBe('platinum')
    expect(getTierByRating(2100)).toBe('diamond')
    expect(getTierByRating(2500)).toBe('master')
  })

  it('calculates ELO rating delta', () => {
    const delta = calculateRatingDelta(1500, 1500)
    expect(delta).toBe(16) // K=32, expected=0.5, delta = 32*0.5 = 16
  })

  it('gives more points for beating higher-rated opponent', () => {
    const deltaLow = calculateRatingDelta(1200, 1800)
    const deltaHigh = calculateRatingDelta(1800, 1200)
    expect(deltaLow).toBeGreaterThan(deltaHigh)
  })

  it('applies match result correctly', () => {
    const winner = { ...basePlayer, playerId: 'w', rating: 1500, tier: 'gold' as const }
    const loser = { ...basePlayer, playerId: 'l', rating: 1500, tier: 'gold' as const }
    const result = applyMatchResult(winner, loser)
    expect(result.winner.rating).toBeGreaterThan(1500)
    expect(result.loser.rating).toBeLessThan(1500)
    expect(result.winner.wins).toBe(11)
    expect(result.loser.losses).toBe(6)
    expect(result.winner.winStreak).toBe(1)
    expect(result.loser.winStreak).toBe(0)
  })

  it('match range is ±200', () => {
    const range = getMatchRange(1500)
    expect(range.min).toBe(1300)
    expect(range.max).toBe(1700)
  })

  it('checks if target is in match range', () => {
    expect(isInMatchRange(1500, 1600)).toBe(true)
    expect(isInMatchRange(1500, 1800)).toBe(false)
  })

  it('calculates win rate', () => {
    expect(getWinRate({ wins: 10, losses: 5 })).toBe(0.67)
    expect(getWinRate({ wins: 0, losses: 0 })).toBe(0)
    expect(getWinRate({ wins: 10, losses: 0 })).toBe(1)
  })

  it('rating does not go below 0', () => {
    const winner = { ...basePlayer, playerId: 'w', rating: 2000, tier: 'platinum' as const }
    const loser = { ...basePlayer, playerId: 'l', rating: 10, tier: 'bronze' as const }
    const result = applyMatchResult(winner, loser)
    expect(result.loser.rating).toBeGreaterThanOrEqual(0)
  })
})
