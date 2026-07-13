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
  findSpeciesIdByLabel,
  speciesGroupOf,
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

  it('36 个内置物种均已认证为可捕获', () => {
    const cap = capturableSpeciesIds()
    expect(cap).toHaveLength(36)
    expect(cap.slice(0, 5)).toEqual(['cat', 'dog', 'rabbit', 'horse', 'cow'])
    expect(cap.at(-1)).toBe('other_animal')
    expect(isCapturableSpecies('rabbit')).toBe(true)
    expect(encyclopediaSpeciesIds()).toContain('rabbit')
    expect(getSpeciesDef('rabbit').status).toBe('capturable')
  })

  it('新增试点无需改业务列表：注册即可进百科', () => {
    const pilot: SpeciesPack = {
      id: 'wombat',
      version: '0.1.0',
      contentId: 'species.wombat',
      status: 'catalog_only',
      names: { common: { 'zh-CN': '袋熊', en: 'Wombat' } },
      welfare: { level: 'wildlife' },
      protection: { status: 'none' },
      assets: { emoji: '🐾' },
    }
    speciesRegistry.register(pilot)
    expect(encyclopediaSpeciesIds()).toContain('wombat')
    expect(isCapturableSpecies('wombat')).toBe(false)
    expect(capturableSpeciesIds()).not.toContain('wombat')
    // 清理，避免污染后续用例
    speciesRegistry.unregister('wombat')
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

  it('SPECIES_DEFS 视图包含完整物种表', () => {
    const defs = buildSpeciesDefs()
    expect(Object.keys(defs)).toHaveLength(36)
    expect(defs.rabbit.name).toBe('兔')
    expect(defs.big_cat.name).toBe('大型猫科动物')
  })

  it('按内容包 aliases/contains 归一化标签', () => {
    expect(findSpeciesIdByLabel('a mallard duck')).toBe('duck')
    expect(findSpeciesIdByLabel('large cat in grass')).toBe('big_cat')
    expect(findSpeciesIdByLabel('generic bird')).toBe('bird')
    expect(findSpeciesIdByLabel('mongoose')).toBeNull()
  })

  it('中文单字不做任意子串匹配，精确物种别名优先', () => {
    expect(findSpeciesIdByLabel('海马')).toBe('fish')
    expect(findSpeciesIdByLabel('一只海马')).toBe('fish')
    expect(findSpeciesIdByLabel('牛蛙')).toBe('frog')
    expect(findSpeciesIdByLabel('食人鱼')).toBe('fish')
    expect(findSpeciesIdByLabel('河马')).toBeNull()
    expect(findSpeciesIdByLabel('蜗牛')).toBeNull()
    expect(findSpeciesIdByLabel('海牛')).toBeNull()
    expect(findSpeciesIdByLabel('木马')).toBeNull()
  })

  it('英文 contains 只匹配完整词', () => {
    expect(findSpeciesIdByLabel('seahorse')).toBe('fish')
    expect(findSpeciesIdByLabel('a horse in grass')).toBe('horse')
    expect(findSpeciesIdByLabel('workhorse')).toBeNull()
    expect(findSpeciesIdByLabel('catfish')).toBeNull()
    expect(findSpeciesIdByLabel('caracal')).toBeNull()
  })

  it('uses descriptor groups and safely groups unknown IDs as other', () => {
    expect(speciesGroupOf('cat')).toBe('companion')
    expect(speciesGroupOf('whale')).toBe('aquatic')
    expect(speciesGroupOf('crab')).toBe('insect')
    expect(speciesGroupOf('other_animal')).toBe('other')
    expect(speciesGroupOf('legacy-unknown')).toBe('other')
  })
})
