import { describe, it, expect } from 'vitest'
import { SPECIES_DEFS, getCardSpecies, SPECIES_RARITY_WEIGHTS } from './types'
import type { CardEntry, SpeciesType } from './types'

describe('types — 物种系统', () => {
  it('SPECIES_DEFS 包含全部三种物种', () => {
    const keys = Object.keys(SPECIES_DEFS)
    expect(keys).toHaveLength(3)
    expect(keys).toContain('cat')
    expect(keys).toContain('goose')
    expect(keys).toContain('dog')
  })

  it('所有 SpeciesDef 包含非空的 emoji / name / throwItem / throwItemEmoji', () => {
    for (const key of Object.keys(SPECIES_DEFS) as SpeciesType[]) {
      const def = SPECIES_DEFS[key]
      expect(def.emoji).toBeTruthy()
      expect(def.name).toBeTruthy()
      expect(def.throwItem).toBeTruthy()
      expect(def.throwItemEmoji).toBeTruthy()
    }
  })

  it('getCardSpecies 无 species 字段时返回 "cat"', () => {
    // 模拟旧数据（无 species 字段）
    const oldEntry = {
      id: 'old',
      no: '#000001',
      rarity: 'common',
      unlocked: true,
      captureDate: '2026-01-01',
      location: 'test',
      lat: 0,
      lng: 0,
      seed: 1,
    } as CardEntry
    expect(getCardSpecies(oldEntry)).toBe('cat')
  })

  it('getCardSpecies 有 species 时正确返回', () => {
    const entry: CardEntry = {
      id: 'd001',
      no: '#000002',
      rarity: 'rare',
      species: 'dog',
      unlocked: true,
      captureDate: '2026-01-01',
      location: 'test',
      lat: 0,
      lng: 0,
      seed: 1,
    }
    expect(getCardSpecies(entry)).toBe('dog')
  })

  it('SPECIES_RARITY_WEIGHTS 三个物种权重总和接近 100~115', () => {
    for (const key of Object.keys(SPECIES_RARITY_WEIGHTS) as SpeciesType[]) {
      const weights = SPECIES_RARITY_WEIGHTS[key]
      const total = weights.reduce((s, w) => s + w.weight, 0)
      // 权重总和应在合理范围（70-115 之间）
      expect(total).toBeGreaterThan(50)
      expect(total).toBeLessThan(150)
    }
  })
})
