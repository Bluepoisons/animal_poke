import { describe, expect, it } from 'vitest'
import {
  BATTLE_ARCHETYPES,
  BATTLE_SKILLS,
  RECOMMENDED_TEAMS,
  assertCatalogMinimums,
} from './designCatalog'

describe('AP-102 battle design catalog', () => {
  it('meets minimum content counts', () => {
    const m = assertCatalogMinimums()
    expect(m.skills).toBeGreaterThanOrEqual(12)
    expect(m.archetypes).toBeGreaterThanOrEqual(6)
    expect(m.teams).toBeGreaterThanOrEqual(3)
  })

  it('recommended teams counter known archetypes', () => {
    const archIds = new Set(BATTLE_ARCHETYPES.map((a) => a.id))
    for (const team of RECOMMENDED_TEAMS) {
      expect(team.roles.length).toBeGreaterThanOrEqual(2)
      expect(team.skillIds.length).toBeGreaterThanOrEqual(3)
      for (const c of team.counters) {
        expect(archIds.has(c)).toBe(true)
      }
    }
    const skillIds = new Set(BATTLE_SKILLS.map((s) => s.id))
    expect(skillIds.has('energy_burst')).toBe(true)
  })
})
