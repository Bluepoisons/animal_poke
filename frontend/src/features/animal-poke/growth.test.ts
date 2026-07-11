import { describe, expect, it } from 'vitest'
import {
  COMPANION_THRESHOLDS,
  FORBIDDEN_GROWTH_KINDS,
  GROWTH_RULES,
  companionLevel,
  countVisibleNodes,
  eventKindForAction,
  hasMinVisibleNodes,
  isForbiddenGrowthKind,
  researcherLevel,
} from './growth'

describe('AP-099 growth helpers', () => {
  it('enforces product rules: no decay / paid power / feeding', () => {
    expect(GROWTH_RULES.noDecay).toBe(true)
    expect(GROWTH_RULES.noPaidPower).toBe(true)
    expect(GROWTH_RULES.noRealWorldFeeding).toBe(true)
    expect(GROWTH_RULES.combatStatsUnchanged).toBe(true)
    expect(GROWTH_RULES.minVisibleNodesPerCompanion).toBeGreaterThanOrEqual(3)
    expect(FORBIDDEN_GROWTH_KINDS).toContain('paid_power')
    expect(FORBIDDEN_GROWTH_KINDS).toContain('decay')
    expect(FORBIDDEN_GROWTH_KINDS).toContain('feed')
    expect(isForbiddenGrowthKind('paid_power')).toBe(true)
    expect(isForbiddenGrowthKind('photo_capture')).toBe(false)
  })

  it('computes researcher and companion levels from thresholds', () => {
    expect(researcherLevel(0)).toBe(0)
    expect(researcherLevel(20)).toBe(1)
    expect(researcherLevel(50)).toBe(2)
    expect(companionLevel(0)).toBe(0)
    expect(companionLevel(10)).toBe(1)
    expect(companionLevel(25)).toBe(2)
    expect(COMPANION_THRESHOLDS.length).toBeGreaterThan(3)
  })

  it('requires at least 3 visible growth nodes', () => {
    const nodes = [
      { visible: true },
      { visible: true },
      { visible: true },
      { visible: false },
    ]
    expect(countVisibleNodes(nodes)).toBe(3)
    expect(hasMinVisibleNodes(nodes)).toBe(true)
    expect(hasMinVisibleNodes([{ visible: true }, { visible: true }])).toBe(false)
  })

  it('maps actions to non-combat event kinds', () => {
    expect(eventKindForAction('capture_first')).toBe('species_first')
    expect(eventKindForAction('capture_repeat')).toBe('species_research')
    expect(eventKindForAction('companion_interact')).toBe('companion_interact')
    expect(eventKindForAction('safe_explore')).toBe('safe_explore')
  })
})
