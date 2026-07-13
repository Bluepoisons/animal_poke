import { describe, it, expect, vi, beforeEach } from 'vitest'
import type { BattlePet, BattleStats } from './types'
import type { RarityTier, SpeciesType, CardEntry } from '../types'
import {
  seedVariance,
  pickElement,
  computeBattleStats,
  applyWeatherModifier,
  getWeatherElementBonus,
  getElementMultiplier,
  applyStrategy,
  computeDamage,
  executeRound,
  executeUltimate,
  checkBattleEnd,
  generateEnemy,
  generateEnemyName,
  computeRewards,
  rollItemDrop,
  weightedRandom,
  scaleStats,
  cardEntryToBattlePet,
} from './logic'
import { RARITY_BASE_STATS, SPECIES_STAT_MODIFIERS, STRATEGY_DEFS, MAX_ROUNDS, ULTIMATE_MULTIPLIER, CRIT_MULTIPLIER, MAX_ENERGY, ENERGY_PER_ATTACK, ENERGY_PER_HIT, DIFFICULTY_MULTIPLIERS, ENEMY_RARITY_POOLS } from './constants'

// ===== 辅助：构造 BattlePet =====

function makeBattlePet(overrides: Partial<BattlePet> = {}): BattlePet {
  return {
    id: 'test_pet',
    name: '测试猫',
    emoji: '🐱',
    species: 'cat',
    rarity: 'common',
    element: 'fire',
    stats: { hp: 100, atk: 30, def: 10, spd: 20, crit: 5, eva: 5 },
    baseStats: { hp: 100, atk: 30, def: 10, spd: 20, crit: 5, eva: 5 },
    currentHp: 100,
    energy: 0,
    isPlayer: true,
    strategy: 'balanced',
    ...overrides,
  }
}

function makeEnemyPet(overrides: Partial<BattlePet> = {}): BattlePet {
  return makeBattlePet({
    id: 'test_enemy',
    name: '测试鹅',
    emoji: '🪿',
    species: 'goose',
    element: 'water',
    isPlayer: false,
    ...overrides,
  })
}

// ===== seedVariance 测试 =====

describe('seedVariance', () => {
  it('返回值在 0.85~1.15 范围内', () => {
    for (let seed = 0; seed < 100; seed++) {
      for (let salt = 0; salt < 10; salt++) {
        const v = seedVariance(seed, salt)
        expect(v).toBeGreaterThanOrEqual(0.85)
        expect(v).toBeLessThanOrEqual(1.15)
      }
    }
  })

  it('确定性：同一 seed+salt 结果一致', () => {
    for (let i = 0; i < 10; i++) {
      expect(seedVariance(42, 1)).toBe(seedVariance(42, 1))
      expect(seedVariance(42, 2)).toBe(seedVariance(42, 2))
    }
  })
})

// ===== computeBattleStats 测试 =====

describe('computeBattleStats', () => {
  it('common cat 有正确的基础属性', () => {
    const stats = computeBattleStats('common', 'cat', 1)
    const base = RARITY_BASE_STATS['common']
    const mod = SPECIES_STAT_MODIFIERS['cat']
    // HP = 60 * 0.8 * variance ≈ 48~55
    expect(stats.hp).toBeGreaterThanOrEqual(Math.round(base.hp * mod.hp * 0.85))
    expect(stats.hp).toBeLessThanOrEqual(Math.round(base.hp * mod.hp * 1.15))
    // SPD = 15 * 1.3 * variance ≈ 16~23
    expect(stats.spd).toBeGreaterThanOrEqual(Math.round(base.spd * mod.spd * 0.85))
    expect(stats.spd).toBeLessThanOrEqual(Math.round(base.spd * mod.spd * 1.15))
  })

  it('legendary dog 有高 HP 和 ATK', () => {
    const stats = computeBattleStats('legendary', 'dog', 100)
    const base = RARITY_BASE_STATS['legendary']
    const mod = SPECIES_STAT_MODIFIERS['dog']
    // HP = 350 * 1.3 * variance ≈ 385~455
    expect(stats.hp).toBeGreaterThanOrEqual(Math.round(base.hp * mod.hp * 0.85))
    expect(stats.hp).toBeLessThanOrEqual(Math.round(base.hp * mod.hp * 1.15))
    // ATK = 95 * 1.2 * variance ≈ 96~131
    expect(stats.atk).toBeGreaterThanOrEqual(Math.round(base.atk * mod.atk * 0.85))
    expect(stats.atk).toBeLessThanOrEqual(Math.round(base.atk * mod.atk * 1.15))
  })

  it('rare goose 有高 DEF', () => {
    const gooseStats = computeBattleStats('rare', 'goose', 5)
    const catStats = computeBattleStats('rare', 'cat', 5)
    // goose DEF = 35 * 1.4 = 49 (× variance)，应高于 cat DEF = 35 * 0.9 = 31.5 (× variance)
    // 由于 variance 相同 seed/salt，确定性对比
    const gooseDefBase = RARITY_BASE_STATS['rare'].def * SPECIES_STAT_MODIFIERS['goose'].def
    const catDefBase = RARITY_BASE_STATS['rare'].def * SPECIES_STAT_MODIFIERS['cat'].def
    expect(gooseDefBase).toBeGreaterThan(catDefBase)
  })

  it('同 seed 同 rarity 同 species 结果一致', () => {
    const s1 = computeBattleStats('epic', 'dog', 999)
    const s2 = computeBattleStats('epic', 'dog', 999)
    expect(s1).toEqual(s2)
  })

  it('cat 的 SPD 始终高于同稀有度 dog（同一 seed）', () => {
    for (const rarity of ['common', 'uncommon', 'rare', 'epic', 'legendary'] as RarityTier[]) {
      const catStats = computeBattleStats(rarity, 'cat', 42)
      const dogStats = computeBattleStats(rarity, 'dog', 42)
      // cat spd modifier = 1.3, dog spd modifier = 0.8，同一 seed 浮动系数相同
      // 所以 cat.spd 一定高于 dog.spd
      expect(catStats.spd).toBeGreaterThan(dogStats.spd)
    }
  })

  it('dog 的 HP 始终高于同稀有度 cat（同一 seed）', () => {
    for (const rarity of ['common', 'uncommon', 'rare', 'epic', 'legendary'] as RarityTier[]) {
      const dogStats = computeBattleStats(rarity, 'dog', 42)
      const catStats = computeBattleStats(rarity, 'cat', 42)
      expect(dogStats.hp).toBeGreaterThan(catStats.hp)
    }
  })

  it('goose 的 DEF 始终高于同稀有度 cat（同一 seed）', () => {
    for (const rarity of ['common', 'uncommon', 'rare', 'epic', 'legendary'] as RarityTier[]) {
      const gooseStats = computeBattleStats(rarity, 'goose', 42)
      const catStats = computeBattleStats(rarity, 'cat', 42)
      expect(gooseStats.def).toBeGreaterThan(catStats.def)
    }
  })
})

// ===== 元素克制测试 =====

describe('getElementMultiplier', () => {
  it('fire vs grass = 1.5（火克草）', () => {
    expect(getElementMultiplier('fire', 'grass')).toBe(1.5)
  })

  it('grass vs fire = 0.67（草被火克）', () => {
    expect(getElementMultiplier('grass', 'fire')).toBe(0.67)
  })

  it('light vs dark = 1.5（光暗互克）', () => {
    expect(getElementMultiplier('light', 'dark')).toBe(1.5)
  })

  it('dark vs light = 1.5（暗克光）', () => {
    expect(getElementMultiplier('dark', 'light')).toBe(1.5)
  })

  it('fire vs fire = 1.0（同元素）', () => {
    expect(getElementMultiplier('fire', 'fire')).toBe(1.0)
  })

  it('fire vs light = 1.0（无克制关系）', () => {
    expect(getElementMultiplier('fire', 'light')).toBe(1.0)
  })
})

// ===== 伤害计算测试 =====

describe('computeDamage', () => {
  beforeEach(() => {
    vi.spyOn(Math, 'random').mockReturnValue(0.5) // 不触发闪避/暴击
  })

  it('基础伤害 = max(1, ATK - DEF)', () => {
    const attacker = makeBattlePet({ stats: { hp: 100, atk: 30, def: 10, spd: 20, crit: 0, eva: 0 } })
    const defender = makeEnemyPet({ stats: { hp: 100, atk: 10, def: 10, spd: 20, crit: 0, eva: 0 } })
    const result = computeDamage(attacker, defender, 'balanced', 'sunny')
    // ATK 30 - DEF 10 = 20, 无克制 fire vs water = 0.67, 暴击否, 天气 sunny fire +10%
    // 20 * 0.67 * (1 + 0.1) = 14.74 → 15
    expect(result.damage).toBe(15)
    expect(result.isMiss).toBe(false)
  })

  it('元素克制 1.5x 伤害', () => {
    const attacker = makeBattlePet({ element: 'fire', stats: { hp: 100, atk: 30, def: 10, spd: 20, crit: 0, eva: 0 } })
    const defender = makeEnemyPet({ element: 'grass', stats: { hp: 100, atk: 10, def: 10, spd: 20, crit: 0, eva: 0 } })
    const result = computeDamage(attacker, defender, 'balanced', 'cloudy')
    // 20 * 1.5 = 30, 无天气修正
    expect(result.damage).toBe(30)
  })

  it('闪避时伤害为 0', () => {
    vi.spyOn(Math, 'random').mockReturnValue(0.01) // < 5%, 触发闪避
    const defender = makeEnemyPet({ stats: { hp: 100, atk: 10, def: 10, spd: 20, crit: 0, eva: 5 } })
    const result = computeDamage(makeBattlePet(), defender, 'balanced', 'sunny')
    expect(result.damage).toBe(0)
    expect(result.isMiss).toBe(true)
  })

  it('必杀技不可被闪避', () => {
    vi.spyOn(Math, 'random').mockReturnValue(0.01) // 即使闪避概率满足
    const defender = makeEnemyPet({ stats: { hp: 100, atk: 10, def: 10, spd: 20, crit: 0, eva: 90 } })
    const result = computeDamage(makeBattlePet(), defender, 'balanced', 'sunny', true)
    expect(result.isMiss).toBe(false)
    expect(result.damage).toBeGreaterThan(0)
  })

  it('必杀技必定暴击', () => {
    vi.spyOn(Math, 'random').mockReturnValue(0.99) // 即使暴击概率不满足
    const result = computeDamage(makeBattlePet(), makeEnemyPet(), 'balanced', 'sunny', true)
    expect(result.isCrit).toBe(true)
  })

  it('策略修正正确应用', () => {
    vi.spyOn(Math, 'random').mockReturnValue(0.5)
    // aggressive: ATK ×1.1, DEF ×0.9
    const aggressiveResult = computeDamage(
      makeBattlePet({ stats: { hp: 100, atk: 30, def: 10, spd: 20, crit: 0, eva: 0 } }),
      makeEnemyPet({ stats: { hp: 100, atk: 10, def: 10, spd: 20, crit: 0, eva: 0 } }),
      'aggressive',
      'cloudy'
    )
    // ATK = 30 * 1.1 = 33, DEF = 10 * 0.9 = 9, base = 33 - 9 = 24, fire vs water = 0.67
    // 24 * 0.67 = 16.08 → 16
    expect(aggressiveResult.damage).toBe(16)

    // defensive: ATK ×0.9, DEF ×1.1
    const defensiveResult = computeDamage(
      makeBattlePet({ stats: { hp: 100, atk: 30, def: 10, spd: 20, crit: 0, eva: 0 } }),
      makeEnemyPet({ stats: { hp: 100, atk: 10, def: 10, spd: 20, crit: 0, eva: 0 } }),
      'defensive',
      'cloudy'
    )
    // ATK = 30 * 0.9 = 27, DEF = 10 * 1.1 = 11, base = 27 - 11 = 16, fire vs water = 0.67
    // 16 * 0.67 = 10.72 → 11
    expect(defensiveResult.damage).toBe(11)
  })
})

// ===== 回合执行测试 =====

describe('executeRound', () => {
  beforeEach(() => {
    vi.spyOn(Math, 'random').mockReturnValue(0.5) // 不闪避/不暴击
  })

  it('SPD 高者先手', () => {
    const player = makeBattlePet({ stats: { hp: 200, atk: 30, def: 10, spd: 50, crit: 0, eva: 0 } })
    const enemy = makeEnemyPet({ stats: { hp: 200, atk: 10, def: 5, spd: 20, crit: 0, eva: 0 } })
    const result = executeRound(player, enemy, 'balanced', 'cloudy', 1)
    // 日志第一条应该是 "我方" 先手攻击
    expect(result.logs[0].text).toContain('我方')
  })

  it('回合后双方能量增加', () => {
    const player = makeBattlePet({ stats: { hp: 200, atk: 30, def: 10, spd: 50, crit: 0, eva: 0 }, energy: 0 })
    const enemy = makeEnemyPet({ stats: { hp: 200, atk: 10, def: 5, spd: 20, crit: 0, eva: 0 }, energy: 0 })
    const result = executeRound(player, enemy, 'balanced', 'cloudy', 1)
    // 攻击方 +15, 受击方 +10，双方都攻击过，所以各 +15 +10 = 25
    expect(result.player.energy).toBe(ENERGY_PER_ATTACK + ENERGY_PER_HIT) // 25
    expect(result.enemy.energy).toBe(ENERGY_PER_ATTACK + ENERGY_PER_HIT) // 25
  })

  it('防守方死亡后不执行后手攻击', () => {
    // 让先手能一击击杀：ATK 远大于 DEF 和 HP
    const player = makeBattlePet({ stats: { hp: 200, atk: 200, def: 10, spd: 100, crit: 0, eva: 0 } })
    const enemy = makeEnemyPet({ stats: { hp: 10, atk: 5, def: 5, spd: 10, crit: 0, eva: 0 } })
    const result = executeRound(player, enemy, 'balanced', 'cloudy', 1)
    // 先手击杀，后手不行动 → 只有 1 条日志
    expect(result.logs.length).toBe(1)
    expect(result.enemy.currentHp).toBe(0)
  })
})

// ===== 战斗结束判定 =====

describe('checkBattleEnd', () => {
  it('player HP=0 返回 lose', () => {
    const player = makeBattlePet({ currentHp: 0 })
    const enemy = makeEnemyPet({ currentHp: 50 })
    expect(checkBattleEnd(player, enemy, 1)).toBe('lose')
  })

  it('enemy HP=0 返回 win', () => {
    const player = makeBattlePet({ currentHp: 50 })
    const enemy = makeEnemyPet({ currentHp: 0 })
    expect(checkBattleEnd(player, enemy, 1)).toBe('win')
  })

  it('回合达到上限返回 draw', () => {
    const player = makeBattlePet({ currentHp: 50 })
    const enemy = makeEnemyPet({ currentHp: 50 })
    expect(checkBattleEnd(player, enemy, MAX_ROUNDS)).toBe('draw')
  })

  it('双方存活且回合未满返回 null', () => {
    const player = makeBattlePet({ currentHp: 50 })
    const enemy = makeEnemyPet({ currentHp: 50 })
    expect(checkBattleEnd(player, enemy, 5)).toBeNull()
  })
})

// ===== 对手生成测试 =====

describe('generateEnemy', () => {
  it('Lv.1 对手稀有度只含 common/uncommon/rare', () => {
    const lowPool = ENEMY_RARITY_POOLS['low']
    const tiers = lowPool.map((p: { tier: RarityTier; weight: number }) => p.tier)
    expect(tiers).not.toContain('epic')
    expect(tiers).not.toContain('legendary')
  })

  it('Lv.10 对手稀有度含 epic/legendary', () => {
    const bossPool = ENEMY_RARITY_POOLS['boss']
    const tiers = bossPool.map((p: { tier: RarityTier; weight: number }) => p.tier)
    expect(tiers).toContain('epic')
    expect(tiers).toContain('legendary')
  })

  it('对手属性随等级提升而增强（难度倍率）', () => {
    const lowMultiplier = DIFFICULTY_MULTIPLIERS['low']
    const bossMultiplier = DIFFICULTY_MULTIPLIERS['boss']
    expect(bossMultiplier).toBeGreaterThan(lowMultiplier)
  })
})

// ===== 奖励计算测试 =====

describe('computeRewards', () => {
  it('胜利获得金币（基于对手稀有度）', () => {
    const commonReward = computeRewards('win', 'common', 1.0)
    expect(commonReward.gold).toBe(15)

    const legendaryReward = computeRewards('win', 'legendary', 1.0)
    expect(legendaryReward.gold).toBe(120)
  })

  it('失败获得 5 金币安慰奖', () => {
    const result = computeRewards('lose', 'legendary', 1.0)
    expect(result.gold).toBe(5)
  })

  it('平局获得 10 金币', () => {
    const result = computeRewards('draw', 'legendary', 1.0)
    expect(result.gold).toBe(10)
  })

  it('胜利金币受难度倍率影响', () => {
    const base = computeRewards('win', 'rare', 1.0)
    const scaled = computeRewards('win', 'rare', 1.3)
    expect(scaled.gold).toBe(Math.round(base.gold * 1.3))
  })
})

// ===== 道具掉落测试 =====

describe('rollItemDrop', () => {
  beforeEach(() => {
    vi.restoreAllMocks() // 恢复 Math.random，让 rollItemDrop 使用真实随机
  })

  it('概率在合理范围（1000 次统计）', () => {
    let drops = 0
    for (let i = 0; i < 1000; i++) {
      if (rollItemDrop()) drops++
    }
    // 15% 概率，1000 次统计期望 150，容差 ±5%
    const rate = drops / 1000
    expect(rate).toBeGreaterThan(0.10)
    expect(rate).toBeLessThan(0.20)
  })
})

// ===== applyWeatherModifier 测试 =====

describe('applyWeatherModifier', () => {
  it('晴天全属性 +5%', () => {
    const stats: BattleStats = { hp: 100, atk: 30, def: 10, spd: 20, crit: 5, eva: 5 }
    const result = applyWeatherModifier(stats, 'sunny')
    expect(result.hp).toBe(105)
    expect(result.atk).toBe(32) // 30 * 1.05 = 31.5 → 32
    expect(result.def).toBe(11) // 10 * 1.05 = 10.5 → 11
    expect(result.crit).toBe(5)  // crit 不受天气修正
    expect(result.eva).toBe(5)   // eva 不受天气修正
  })
})

// ===== cardEntryToBattlePet 测试 =====

describe('cardEntryToBattlePet', () => {
  it('正确转换 CardEntry 为 BattlePet', () => {
    const entry: CardEntry = {
      id: 'c001',
      no: '#000059',
      rarity: 'common',
      species: 'cat',
      unlocked: true,
      captureDate: '2026-07-08',
      location: '海曙区',
      lat: 29.87,
      lng: 121.55,
      seed: 1,
      isNew: true,
    }
    const pet = cardEntryToBattlePet(entry, 'sunny')
    expect(pet.id).toBe('c001')
    expect(pet.species).toBe('cat')
    expect(pet.element).toBe(pickElement(1))
    expect(pet.isPlayer).toBe(true)
    expect(pet.currentHp).toBe(pet.stats.hp)
    expect(pet.energy).toBe(0)
  })

  it('does not turn a legacy entry without species into a cat', () => {
    const entry: CardEntry = {
      id: 'legacy',
      no: '#LEGACY',
      rarity: 'common',
      unlocked: true,
      captureDate: '2026-07-08',
      location: '未知',
      lat: 0,
      lng: 0,
      seed: 1,
    }
    const pet = cardEntryToBattlePet(entry, 'sunny')
    expect(pet.species).toBe('unknown')
    expect(pet.name).toBe('动物伙伴')
  })
})

// ===== applyStrategy 测试 =====

describe('applyStrategy', () => {
  it('aggressive: ATK ×1.1, DEF ×0.9', () => {
    const result = applyStrategy(30, 10, 'aggressive')
    expect(result.atk).toBe(33)
    expect(result.def).toBe(9)
  })

  it('balanced: 不修正', () => {
    const result = applyStrategy(30, 10, 'balanced')
    expect(result.atk).toBe(30)
    expect(result.def).toBe(10)
  })

  it('defensive: ATK ×0.9, DEF ×1.1', () => {
    const result = applyStrategy(30, 10, 'defensive')
    expect(result.atk).toBe(27)
    expect(result.def).toBe(11)
  })
})

// ===== pickElement 测试 =====

describe('pickElement', () => {
  it('确定性：同一 seed 结果一致', () => {
    for (let i = 0; i < 10; i++) {
      expect(pickElement(42)).toBe(pickElement(42))
    }
  })

  it('seed % 5 映射到正确元素', () => {
    expect(pickElement(0)).toBe('fire')   // 0 % 5 = 0 → fire
    expect(pickElement(1)).toBe('water')  // 1 % 5 = 1 → water
    expect(pickElement(2)).toBe('grass')  // 2 % 5 = 2 → grass
    expect(pickElement(3)).toBe('light')  // 3 % 5 = 3 → light
    expect(pickElement(4)).toBe('dark')   // 4 % 5 = 4 → dark
    expect(pickElement(5)).toBe('fire')   // 5 % 5 = 0 → fire
  })
})

// ===== scaleStats 测试 =====

describe('scaleStats', () => {
  it('按倍率缩放所有属性', () => {
    const stats: BattleStats = { hp: 100, atk: 30, def: 10, spd: 20, crit: 5, eva: 5 }
    const result = scaleStats(stats, 1.2)
    expect(result.hp).toBe(120)
    expect(result.atk).toBe(36)
    expect(result.def).toBe(12)
    expect(result.spd).toBe(24)
    expect(result.crit).toBe(6)
    expect(result.eva).toBe(6)
  })
})

// ===== executeUltimate 测试 =====

describe('executeUltimate', () => {
  beforeEach(() => {
    vi.spyOn(Math, 'random').mockReturnValue(0.5)
  })

  it('必杀技清零攻击方能量', () => {
    const attacker = makeBattlePet({ energy: 100 })
    const defender = makeEnemyPet()
    const result = executeUltimate(attacker, defender, 'balanced', 'cloudy', 1, '我方')
    expect(result.attacker.energy).toBe(0)
  })

  it('必杀技造成高伤害（1.8x + 必定暴击）', () => {
    const attacker = makeBattlePet({ stats: { hp: 100, atk: 30, def: 10, spd: 20, crit: 0, eva: 0 }, energy: 100 })
    const defender = makeEnemyPet({ stats: { hp: 200, atk: 10, def: 10, spd: 20, crit: 0, eva: 0 }, element: 'grass' })
    const result = executeUltimate(attacker, defender, 'balanced', 'cloudy', 1, '我方')
    // 基础伤害 = 30 - 10 = 20, fire vs grass = 1.5, 必杀暴击 1.8x
    // 20 * 1.5 * 1.8 = 54
    expect(result.log.text).toContain('54')
    expect(result.log.type).toBe('ultimate')
  })
})
