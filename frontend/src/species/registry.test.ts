import { describe, it, expect } from 'vitest'
import {
  SPECIES_PACKS,
  buildSpeciesDefs,
  capturableSpeciesIds,
  encyclopediaSpeciesIds,
  isCapturableSpecies,
  getSpeciesDef,
  getStatModifiers,
  effectiveStatus,
  validatePackSchema,
  speciesRegistry,
} from './index'
import type { SpeciesPack } from './types'

describe('species packs (AP-093)', () => {
  it('schema: 内置包通过校验', () => {
    for (const p of SPECIES_PACKS) {
      expect(validatePackSchema(p), p.id).toEqual([])
    }
  })

  it('别名不冲突：id 唯一', () => {
    const ids = SPECIES_PACKS.map((p) => p.id)
    expect(new Set(ids).size).toBe(ids.length)
  })

  it('交叉引用：contentId 与 version 齐全', () => {
    for (const p of SPECIES_PACKS) {
      expect(p.contentId).toBe(`species.${p.id}`)
      expect(p.version).toMatch(/^\d+\.\d+\.\d+$/)
    }
  })

  it('可捕获列表不含未认证试点', () => {
    const cap = capturableSpeciesIds()
    expect(cap).toEqual(['cat', 'dog', 'goose'])
    expect(isCapturableSpecies('rabbit')).toBe(false)
    expect(encyclopediaSpeciesIds()).toContain('rabbit')
    expect(getSpeciesDef('rabbit').status).toBe('catalog_only')
  })

  it('新增试点无需改业务列表：注册即可进百科', () => {
    const pilot: SpeciesPack = {
      id: 'squirrel',
      version: '0.1.0',
      contentId: 'species.squirrel',
      status: 'catalog_only',
      names: { common: { 'zh-CN': '松鼠', en: 'Squirrel' } },
      welfare: { level: 'wildlife' },
      protection: { status: 'none' },
      assets: { emoji: '🐿️' },
    }
    speciesRegistry.register(pilot)
    expect(encyclopediaSpeciesIds()).toContain('squirrel')
    expect(isCapturableSpecies('squirrel')).toBe(false)
    expect(capturableSpeciesIds()).not.toContain('squirrel')
    // 清理，避免污染后续用例
    speciesRegistry.unregister('squirrel')
  })

  it('认证过期安全降级为 catalog_only', () => {
    const expired: SpeciesPack = {
      id: 'fox',
      version: '1.0.0',
      contentId: 'species.fox',
      status: 'capturable',
      certification: {
        goldenSetVersion: '1.0.0',
        expiresAt: '2020-01-01T00:00:00Z',
      },
      names: { common: { en: 'Fox' } },
      welfare: { level: 'wildlife' },
      protection: { status: 'none' },
      assets: { emoji: '🦊' },
      gameplay: {
        detectThreshold: 0.9,
        statModifiers: { hp: 1, atk: 1, def: 1, spd: 1, crit: 0, eva: 0 },
        rarityWeights: [{ tier: 'common', weight: 1 }],
      },
    }
    expect(effectiveStatus(expired, new Date('2026-07-11'))).toBe('catalog_only')
  })

  it('缺 gameplay 字段：capturable → recognition_certified', () => {
    const incomplete: SpeciesPack = {
      id: 'fox',
      version: '1.0.0',
      contentId: 'species.fox',
      status: 'capturable',
      certification: { goldenSetVersion: '1.0.0' },
      names: { common: { en: 'Fox' } },
      welfare: { level: 'wildlife' },
      protection: { status: 'none' },
      assets: { emoji: '🦊' },
      gameplay: { detectThreshold: 0.9 },
    }
    expect(effectiveStatus(incomplete)).toBe('recognition_certified')
  })

  it('未知 ID 安全降级定义', () => {
    const def = getSpeciesDef('not-a-species')
    expect(def.emoji).toBeTruthy()
    expect(def.status).toBe('catalog_only')
    expect(getStatModifiers('not-a-species').hp).toBe(1)
  })

  it('SPECIES_DEFS 视图包含第四物种', () => {
    const defs = buildSpeciesDefs()
    expect(Object.keys(defs).sort()).toEqual(['cat', 'dog', 'goose', 'rabbit'])
    expect(defs.rabbit.name).toBe('兔')
  })
})
