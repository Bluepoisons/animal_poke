import { describe, it, expect } from 'vitest'
import {
  chaptersPlayableWithLowCollection,
  getChapter,
  homeModeRouteCoverage,
  listSeasonChapters,
  prerequisitesOf,
  season1Architecture,
  validateSeasonArchitecture,
} from './architecture'

describe('AP-116 season1 architecture', () => {
  it('validates full structural gates', () => {
    expect(validateSeasonArchitecture()).toEqual([])
  })

  it('has ordered prologue, four chapters, finale with independent themes', () => {
    const list = listSeasonChapters()
    expect(list.map((c) => c.order)).toEqual([0, 1, 2, 3, 4, 5])
    const themes = new Set(list.map((c) => c.themeQuestion))
    expect(themes.size).toBe(6)
    expect(season1Architecture.coreTheme).toContain('城市')
  })

  it('dependency graph is linear and unlockable', () => {
    expect(prerequisitesOf('prologue.blank_page')).toEqual([])
    expect(prerequisitesOf('ch01.alley_echo')).toEqual(['prologue.blank_page'])
    expect(prerequisitesOf('finale.who_tells_the_city')).toEqual(['ch04.map_blank'])
  })

  it('every chapter has delayed choice echo and accessibility routes', () => {
    for (const ch of listSeasonChapters()) {
      expect(ch.choice.options.length).toBeGreaterThanOrEqual(2)
      expect(ch.routes.homeMode.length).toBeGreaterThan(0)
      expect(ch.routes.noCamera.length).toBeGreaterThan(0)
      expect(ch.routes.noAnimal.length).toBeGreaterThan(0)
      expect(ch.passableHomeMode).toBe(true)
      expect(ch.passableWithoutRareAnimal).toBe(true)
    }
  })

  it('supports low-collection and home-mode coverage', () => {
    const low = chaptersPlayableWithLowCollection(1)
    expect(low).toContain('prologue.blank_page')
    expect(low).toContain('ch04.map_blank')
    expect(homeModeRouteCoverage()).toHaveLength(6)
  })

  it('outcomes do not depend on collection rate', () => {
    expect(season1Architecture.outcomes.length).toBeGreaterThanOrEqual(2)
    for (const o of season1Architecture.outcomes) {
      expect(o.independentOfCollectionRate).toBe(true)
    }
  })

  it('getChapter resolves known ids', () => {
    expect(getChapter('ch02.along_river_sleepless')?.title).toContain('沿河')
  })
})
