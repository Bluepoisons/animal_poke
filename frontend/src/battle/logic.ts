import type { RarityTier, SpeciesType, CardEntry } from '../types'
import { resolveSpeciesDef } from '../types'
import type { ElementType, StrategyType, WeatherType, BattleStats, BattlePet, BattleLogEntry, BattleResult, BattleRewards } from './types'
import {
  RARITY_BASE_STATS,
  SPECIES_STAT_MODIFIERS,
  ELEMENT_CHART,
  STRATEGY_DEFS,
  ELEMENT_TYPES,
  ENEMY_RARITY_POOLS,
  LEVEL_TO_POOL_MAP,
  DIFFICULTY_MULTIPLIERS,
  BATTLE_GOLD_REWARDS,
  BATTLE_LOSE_GOLD,
  BATTLE_DRAW_GOLD,
  ITEM_DROP_RATE,
  ITEM_DROP_POOL,
  ENEMY_NAME_PREFIXES,
  SPECIES_LIST,
  WEATHER_STAT_MODIFIER,
  WEATHER_ELEMENT_BONUS,
  MAX_ENERGY,
  ENERGY_PER_ATTACK,
  ENERGY_PER_HIT,
  CRIT_MULTIPLIER,
  ULTIMATE_MULTIPLIER,
  MAX_ROUNDS,
} from './constants'
import { getBattleXp } from '../stamina/logic'

// ===== 确定性伪随机 =====

/** 基于 seed 生成 0.85~1.15 的浮动系数（同一 seed+salt 永远相同结果） */
export function seedVariance(seed: number, salt: number): number {
  const x = Math.sin(seed * 9999 + salt * 7777) * 10000
  const frac = x - Math.floor(x)
  return 0.85 + frac * 0.3
}

/** 基于 seed 选取元素 */
export function pickElement(seed: number): ElementType {
  const index = ((seed % 5) + 5) % 5
  return ELEMENT_TYPES[index]
}

// ===== 属性计算 =====

/** 计算六维属性（稀有度基础 × 物种修正 × 个体浮动） */
export function computeBattleStats(rarity: RarityTier, species: SpeciesType, seed: number): BattleStats {
  const base = RARITY_BASE_STATS[rarity]
  const mod = SPECIES_STAT_MODIFIERS[species] ?? { hp: 1, atk: 1, def: 1, spd: 1, crit: 0, eva: 0 }

  return {
    hp:   Math.round(base.hp   * mod.hp   * seedVariance(seed, 1)),
    atk:  Math.round(base.atk  * mod.atk  * seedVariance(seed, 2)),
    def:  Math.round(base.def  * mod.def  * seedVariance(seed, 3)),
    spd:  Math.round(base.spd  * mod.spd  * seedVariance(seed, 4)),
    crit: base.crit + mod.crit,
    eva:  base.eva + mod.eva,
  }
}

/** 应用天气属性修正 */
export function applyWeatherModifier(stats: BattleStats, weather: WeatherType): BattleStats {
  const modifier = WEATHER_STAT_MODIFIER[weather]
  return {
    hp:   Math.round(stats.hp   * modifier),
    atk:  Math.round(stats.atk  * modifier),
    def:  Math.round(stats.def  * modifier),
    spd:  Math.round(stats.spd  * modifier),
    crit: stats.crit,
    eva:  stats.eva,
  }
}

/** 天气元素伤害加成（返回 ±0.1 或 0） */
export function getWeatherElementBonus(weather: WeatherType, element: ElementType): number {
  return WEATHER_ELEMENT_BONUS[weather][element] ?? 0
}

/**
 * 应用状态修正倍率到战斗属性
 * 与 applyWeatherModifier 一致，仅修正 hp/atk/def/spd，不修正 crit/eva
 *
 * @param stats 原始属性（已含天气修正）
 * @param multiplier 状态修正倍率（1.0 = 无修正，0.65 = 感冒 -35%）
 * @returns 修正后的属性
 */
export function applyStatusMultiplier(stats: BattleStats, multiplier: number): BattleStats {
  if (multiplier === 1.0) return stats
  return {
    hp:   Math.round(stats.hp   * multiplier),
    atk:  Math.round(stats.atk  * multiplier),
    def:  Math.round(stats.def  * multiplier),
    spd:  Math.round(stats.spd  * multiplier),
    crit: stats.crit,
    eva:  stats.eva,
  }
}

// ===== 元素克制 =====

/** 获取元素克制倍率 */
export function getElementMultiplier(attacker: ElementType, defender: ElementType): number {
  return ELEMENT_CHART[attacker][defender]
}

// ===== 策略修正 =====

/** 应用策略修正（返回 atk/def 的修正值） */
export function applyStrategy(atk: number, def: number, strategy: StrategyType): { atk: number; def: number } {
  const strat = STRATEGY_DEFS[strategy]
  return {
    atk: Math.round(atk * strat.atkMod),
    def: Math.round(def * strat.defMod),
  }
}

// ===== 伤害计算 =====

/** 计算单次伤害（纯函数，不含随机判定）
 *  注意：闪避和暴击使用 Math.random()，测试时需要 mock */
export function computeDamage(
  attacker: BattlePet,
  defender: BattlePet,
  strategy: StrategyType,
  weather: WeatherType,
  isUltimate: boolean = false
): { damage: number; isCrit: boolean; isMiss: boolean } {
  // 闪避判定（必杀技不可闪避）
  if (!isUltimate && Math.random() * 100 < defender.stats.eva) {
    return { damage: 0, isCrit: false, isMiss: true }
  }

  // 策略修正
  const strat = applyStrategy(attacker.stats.atk, defender.stats.def, strategy)

  // 基础伤害
  let damage = Math.max(1, strat.atk - strat.def)

  // 元素克制
  damage *= getElementMultiplier(attacker.element, defender.element)

  // 天气元素加成
  const weatherBonus = getWeatherElementBonus(weather, attacker.element)
  damage *= (1 + weatherBonus)

  // 暴击判定（必杀技必定暴击）
  const isCrit = isUltimate || Math.random() * 100 < attacker.stats.crit
  if (isCrit) {
    damage *= isUltimate ? ULTIMATE_MULTIPLIER : CRIT_MULTIPLIER
  }

  return { damage: Math.round(damage), isCrit, isMiss: false }
}

// ===== 回合执行 =====

/** 执行一回合（纯函数，返回新状态） */
export function executeRound(
  player: BattlePet,
  enemy: BattlePet,
  strategy: StrategyType,
  weather: WeatherType,
  round: number
): { player: BattlePet; enemy: BattlePet; logs: BattleLogEntry[] } {
  const logs: BattleLogEntry[] = []
  const p = { ...player, stats: { ...player.stats } }
  const e = { ...enemy, stats: { ...enemy.stats } }

  // 判定先后手：SPD 高者先手，相同则随机
  const playerFirst = p.stats.spd > e.stats.spd
    ? true
    : e.stats.spd > p.stats.spd
      ? false
      : Math.random() < 0.5

  // 先手和后手的引用
  const first = playerFirst ? p : e
  const second = playerFirst ? e : p
  const firstLabel = playerFirst ? '我方' : '敌方'
  const secondLabel = playerFirst ? '敌方' : '我方'

  // 先手行动
  if (second.currentHp > 0) {
    const result = computeDamage(first, second, strategy, weather)
    if (result.isMiss) {
      logs.push({ round, text: `${firstLabel}攻击被闪避！`, type: 'miss' })
    } else {
      second.currentHp = Math.max(0, second.currentHp - result.damage)
      logs.push({
        round,
        text: result.isCrit
          ? `${firstLabel}暴击！造成 ${result.damage} 伤害`
          : `${firstLabel}攻击，造成 ${result.damage} 伤害`,
        type: result.isCrit ? 'crit' : 'attack',
      })
    }
    // 积累能量
    first.energy = Math.min(MAX_ENERGY, first.energy + ENERGY_PER_ATTACK)
    second.energy = Math.min(MAX_ENERGY, second.energy + ENERGY_PER_HIT)
  }

  // 后手行动（先手击杀后不执行）
  if (second.currentHp > 0 && first.currentHp > 0) {
    const result = computeDamage(second, first, strategy, weather)
    if (result.isMiss) {
      logs.push({ round, text: `${secondLabel}攻击被闪避！`, type: 'miss' })
    } else {
      first.currentHp = Math.max(0, first.currentHp - result.damage)
      logs.push({
        round,
        text: result.isCrit
          ? `${secondLabel}暴击！造成 ${result.damage} 伤害`
          : `${secondLabel}攻击，造成 ${result.damage} 伤害`,
        type: result.isCrit ? 'crit' : 'attack',
      })
    }
    // 积累能量
    second.energy = Math.min(MAX_ENERGY, second.energy + ENERGY_PER_ATTACK)
    first.energy = Math.min(MAX_ENERGY, first.energy + ENERGY_PER_HIT)
  }

  return { player: p, enemy: e, logs }
}

/** 释放必杀技 */
export function executeUltimate(
  attacker: BattlePet,
  defender: BattlePet,
  strategy: StrategyType,
  weather: WeatherType,
  round: number,
  label: string
): { attacker: BattlePet; defender: BattlePet; log: BattleLogEntry } {
  const a = { ...attacker, stats: { ...attacker.stats } }
  const d = { ...defender, stats: { ...defender.stats } }

  const result = computeDamage(a, d, strategy, weather, true)
  d.currentHp = Math.max(0, d.currentHp - result.damage)
  // 必杀释放后能量清零
  a.energy = 0

  return {
    attacker: a,
    defender: d,
    log: {
      round,
      text: `${label}释放必杀技！造成 ${result.damage} 伤害`,
      type: 'ultimate',
    },
  }
}

// ===== 战斗结束判定 =====

/** 检查战斗是否结束 */
export function checkBattleEnd(player: BattlePet, enemy: BattlePet, round: number): BattleResult {
  if (player.currentHp <= 0) return 'lose'
  if (enemy.currentHp <= 0) return 'win'
  if (round >= MAX_ROUNDS) return 'draw'
  return null
}

// ===== 对手生成 =====

/** 加权随机选取索引 */
export function weightedRandom(items: { weight: number }[]): number {
  const total = items.reduce((sum, item) => sum + item.weight, 0)
  let rand = Math.random() * total
  for (let i = 0; i < items.length; i++) {
    rand -= items[i].weight
    if (rand <= 0) return i
  }
  return items.length - 1
}

/** 按倍率缩放属性 */
export function scaleStats(stats: BattleStats, multiplier: number): BattleStats {
  return {
    hp:   Math.round(stats.hp   * multiplier),
    atk:  Math.round(stats.atk  * multiplier),
    def:  Math.round(stats.def  * multiplier),
    spd:  Math.round(stats.spd  * multiplier),
    crit: Math.round(stats.crit * multiplier),
    eva:  Math.round(stats.eva  * multiplier),
  }
}

/** 生成对手名称 */
export function generateEnemyName(species: SpeciesType): string {
  const prefix = ENEMY_NAME_PREFIXES[Math.floor(Math.random() * ENEMY_NAME_PREFIXES.length)]
  const speciesName = resolveSpeciesDef(species).name
  return `${prefix}·${speciesName}`
}

/** 生成 PvE 对手 */
export function generateEnemy(playerLevel: number): BattlePet {
  // 确定难度池
  const clampedLevel = Math.max(1, Math.min(10, playerLevel))
  const poolKey = LEVEL_TO_POOL_MAP[clampedLevel] ?? 'low'
  const pool = ENEMY_RARITY_POOLS[poolKey]

  // 加权随机选取稀有度
  const rarityIndex = weightedRandom(pool)
  const rarity = pool[rarityIndex].tier

  // 随机物种
  const species = SPECIES_LIST[Math.floor(Math.random() * SPECIES_LIST.length)]

  // 生成随机 seed
  const seed = Math.floor(Math.random() * 100000)

  // 计算属性
  const baseStats = computeBattleStats(rarity, species, seed)

  // 应用难度倍率
  const multiplier = DIFFICULTY_MULTIPLIERS[poolKey]
  const stats = scaleStats(baseStats, multiplier)

  // 随机元素
  const element = pickElement(seed)

  return {
    id: 'enemy_' + Date.now(),
    name: generateEnemyName(species),
    emoji: resolveSpeciesDef(species).emoji,
    species,
    rarity,
    element,
    stats,
    baseStats,
    currentHp: stats.hp,
    energy: 0,
    isPlayer: false,
    strategy: 'balanced',
  }
}

// ===== 奖励计算 =====

/** 计算战斗奖励 */
export function computeRewards(result: BattleResult, enemyRarity: RarityTier, difficultyMultiplier: number): BattleRewards {
  if (result === 'win') {
    const gold = Math.round(BATTLE_GOLD_REWARDS[enemyRarity] * difficultyMultiplier)
    const exp = getBattleXp('win', enemyRarity)
    const droppedItem = rollItemDrop()
    return { gold, exp, droppedItem }
  }
  if (result === 'draw') {
    return { gold: BATTLE_DRAW_GOLD, exp: getBattleXp('draw', enemyRarity) }
  }
  // 失败
  return { gold: BATTLE_LOSE_GOLD, exp: getBattleXp('lose', enemyRarity) }
}

/** 道具掉落判定（15% 概率） */
export function rollItemDrop(): string | undefined {
  if (Math.random() >= ITEM_DROP_RATE) return undefined
  const index = weightedRandom(ITEM_DROP_POOL)
  return ITEM_DROP_POOL[index].itemId
}

// ===== CardEntry 转换 =====

/** 将 CardEntry 转换为 BattlePet */
export function cardEntryToBattlePet(entry: CardEntry, weather: WeatherType): BattlePet {
  const species = entry.species ?? 'cat'
  const element = pickElement(entry.seed)
  const baseStats = computeBattleStats(entry.rarity, species, entry.seed)
  const stats = applyWeatherModifier(baseStats, weather)

  return {
    id: entry.id,
    name: resolveSpeciesDef(species).name,
    emoji: resolveSpeciesDef(species).emoji,
    species,
    rarity: entry.rarity,
    element,
    stats,
    baseStats,
    currentHp: stats.hp,
    energy: 0,
    isPlayer: true,
    strategy: 'balanced',
  }
}
