import { describe, it, expect } from 'vitest'
import {
  SPECIES_DEFS,
  UNKNOWN_SPECIES,
  getCardSpecies,
  normalizeSpeciesId,
  resolveSpeciesDef,
  SPECIES_RARITY_WEIGHTS,
  CAPTURABLE_SPECIES,
  canCaptureSpecies,
} from './types'
import type { CardEntry, SpeciesType } from './types'
import { encyclopediaSpeciesIds } from './species'

describe('types — 物种系统', () => {
  it('SPECIES_DEFS 包含完整的 36 个可捕获物种', () => {
    const keys = Object.keys(SPECIES_DEFS)
    expect(keys).toHaveLength(36)
    expect(keys).toContain('cat')
    expect(keys).toContain('goose')
    expect(keys).toContain('dog')
    expect(keys).toContain('rabbit')
    expect(keys).toContain('bird')
    expect(keys).toContain('whale')
    expect(CAPTURABLE_SPECIES).toHaveLength(36)
    expect(canCaptureSpecies('rabbit')).toBe(true)
    expect(canCaptureSpecies('bird')).toBe(true)
    expect(canCaptureSpecies('other_animal')).toBe(true)
    expect(encyclopediaSpeciesIds()).toContain('rabbit')
  })

  it('所有 SpeciesDef 包含非空的 emoji / name / throwItem / throwItemEmoji', () => {
    for (const key of Object.keys(SPECIES_DEFS) as SpeciesType[]) {
      const def = SPECIES_DEFS[key]
      expect(def.emoji).toBeTruthy()
      expect(def.name).toBeTruthy()
      expect(def.throwItem).toBeTruthy()
      expect(def.throwItemEmoji).toBeTruthy()
      expect(def.contentId).toBeTruthy()
      expect(def.version).toBeTruthy()
    }
  })

  it('getCardSpecies 对缺失或未知 species 使用不可捕获的 unknown', () => {
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
    expect(getCardSpecies(oldEntry)).toBe(UNKNOWN_SPECIES)
    expect(normalizeSpeciesId('fox')).toBe(UNKNOWN_SPECIES)
    expect(normalizeSpeciesId('  ')).toBe(UNKNOWN_SPECIES)
    expect(canCaptureSpecies(UNKNOWN_SPECIES)).toBe(false)
    expect(resolveSpeciesDef(UNKNOWN_SPECIES).name).toBe('动物伙伴')
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

  it('SPECIES_RARITY_WEIGHTS 可捕获物种权重总和在合理范围', () => {
    for (const key of Object.keys(SPECIES_RARITY_WEIGHTS) as SpeciesType[]) {
      const weights = SPECIES_RARITY_WEIGHTS[key]
      const total = weights.reduce((s, w) => s + w.weight, 0)
      expect(total).toBeGreaterThan(50)
      expect(total).toBeLessThan(150)
    }
  })
})
