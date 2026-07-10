import type { RarityTier, SpeciesType } from '../types'
import type { ElementType, StrategyType, WeatherType, BattleStats } from './types'
import type { ItemId } from '../shop/constants'

// AP-032: single source of truth for ItemId lives in shop/constants.
export type { ItemId }

// ===== 稀有度基础属性表 =====
export const RARITY_BASE_STATS: Record<RarityTier, BattleStats> = {
  common:    { hp: 60,  atk: 15, def: 10, spd: 15, crit: 5,  eva: 5  },
  uncommon:  { hp: 100, atk: 28, def: 20, spd: 30, crit: 7,  eva: 7  },
  rare:      { hp: 150, atk: 45, def: 35, spd: 50, crit: 10, eva: 10 },
  epic:      { hp: 220, atk: 65, def: 50, spd: 70, crit: 15, eva: 15 },
  legendary: { hp: 350, atk: 95, def: 68, spd: 88, crit: 20, eva: 20 },
}

// ===== 物种属性修正 =====
export const SPECIES_STAT_MODIFIERS: Record<SpeciesType, {
  hp: number; atk: number; def: number; spd: number; crit: number; eva: number
}> = {
  cat:   { hp: 0.8, atk: 0.9, def: 0.9, spd: 1.3, crit: 10, eva: 5  },
  dog:   { hp: 1.3, atk: 1.2, def: 1.0, spd: 0.8, crit: 3,  eva: 2  },
  goose: { hp: 1.0, atk: 0.8, def: 1.4, spd: 0.9, crit: 2,  eva: 8  },
}

// ===== 元素克制表 =====
export const ELEMENT_CHART: Record<ElementType, Record<ElementType, number>> = {
  fire:   { fire: 1.0, water: 0.67, grass: 1.5, light: 1.0, dark: 1.0 },
  water:  { fire: 1.5, water: 1.0,  grass: 0.67, light: 1.0, dark: 1.0 },
  grass:  { fire: 0.67, water: 1.5, grass: 1.0, light: 1.0, dark: 1.0 },
  light:  { fire: 1.0, water: 1.0, grass: 1.0, light: 1.0, dark: 1.5 },
  dark:   { fire: 1.0, water: 1.0, grass: 1.0, light: 1.5, dark: 1.0 },
}

// ===== 策略定义 =====
export const STRATEGY_DEFS: Record<StrategyType, { atkMod: number; defMod: number; label: string }> = {
  aggressive: { atkMod: 1.1, defMod: 0.9, label: '激进' },
  balanced:   { atkMod: 1.0, defMod: 1.0, label: '平衡' },
  defensive:  { atkMod: 0.9, defMod: 1.1, label: '防守' },
}

// ===== 战斗常量 =====
export const BATTLE_STAMINA_COST = 20
export const MAX_ROUNDS = 30
export const MAX_ENERGY = 100
export const ENERGY_PER_ATTACK = 15
export const ENERGY_PER_HIT = 10
export const CRIT_MULTIPLIER = 2.0
export const ULTIMATE_MULTIPLIER = 1.8
export const AUTO_PLAY_INTERVAL_MS = 1500

export const ELEMENT_TYPES: ElementType[] = ['fire', 'water', 'grass', 'light', 'dark']

// ===== 对手稀有度池 =====
export const ENEMY_RARITY_POOLS: Record<string, { tier: RarityTier; weight: number }[]> = {
  low:    [{ tier: 'common', weight: 70 }, { tier: 'uncommon', weight: 25 }, { tier: 'rare', weight: 5 }],
  mid:    [{ tier: 'common', weight: 30 }, { tier: 'uncommon', weight: 45 }, { tier: 'rare', weight: 20 }, { tier: 'epic', weight: 5 }],
  high:   [{ tier: 'uncommon', weight: 30 }, { tier: 'rare', weight: 40 }, { tier: 'epic', weight: 25 }, { tier: 'legendary', weight: 5 }],
  elite:  [{ tier: 'rare', weight: 30 }, { tier: 'epic', weight: 40 }, { tier: 'legendary', weight: 30 }],
  boss:   [{ tier: 'epic', weight: 40 }, { tier: 'legendary', weight: 60 }],
}

export const LEVEL_TO_POOL_MAP: Record<number, 'low' | 'mid' | 'high' | 'elite' | 'boss'> = {
  1: 'low', 2: 'low',
  3: 'mid', 4: 'mid',
  5: 'high', 6: 'high',
  7: 'elite', 8: 'elite',
  9: 'boss', 10: 'boss',
}

export const DIFFICULTY_MULTIPLIERS: Record<string, number> = {
  low: 0.9, mid: 1.0, high: 1.1, elite: 1.2, boss: 1.3,
}

// ===== 奖励表 =====
export const BATTLE_GOLD_REWARDS: Record<RarityTier, number> = {
  common: 15, uncommon: 25, rare: 40, epic: 70, legendary: 120,
}

export const BATTLE_LOSE_GOLD = 5
export const BATTLE_DRAW_GOLD = 10
export const ITEM_DROP_RATE = 0.15

export const ITEM_DROP_POOL: { itemId: ItemId; weight: number }[] = [
  { itemId: 'toy_ball', weight: 40 },
  { itemId: 'bait', weight: 25 },
  { itemId: 'food_pack', weight: 25 },
  { itemId: 'cold_medicine', weight: 10 },
]

export const ENEMY_NAME_PREFIXES = ['流浪的', '凶猛的', '神秘的', '虚弱的', '狂暴的', '沉睡的', '机警的', '慵懒的']

// ===== 天气修正 =====
export const WEATHER_STAT_MODIFIER: Record<WeatherType, number> = {
  sunny: 1.05, cloudy: 1.0, overcast: 1.0, rainy: 1.0, snowy: 1.0, foggy: 1.0, extreme: 1.0,
}

export const WEATHER_ELEMENT_BONUS: Record<WeatherType, Partial<Record<ElementType, number>>> = {
  sunny:   { fire: 0.1, water: -0.1 },
  cloudy:  {},
  overcast: {},
  rainy:   { water: 0.1, fire: -0.1 },
  snowy:   {},
  foggy:   {},
  extreme: {},
}

// ===== 物种列表 =====
export const SPECIES_LIST: SpeciesType[] = ['cat', 'goose', 'dog']
