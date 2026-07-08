import type { LevelTableRow } from './types'
import type { RarityTier } from '../types'

/** 等级表（索引 0 = Lv.1，索引 9 = Lv.10） */
export const LEVEL_TABLE: LevelTableRow[] = [
  { level: 1,  requiredCaptures: 0,   requiredExp: 0,    maxStamina: 120, rewardGold: 0,    shopBonus: 0,   isMaxLevel: false },
  { level: 2,  requiredCaptures: 10,  requiredExp: 100,  maxStamina: 134, rewardGold: 100,  shopBonus: 2,   isMaxLevel: false },
  { level: 3,  requiredCaptures: 25,  requiredExp: 250,  maxStamina: 148, rewardGold: 150,  shopBonus: 4,   isMaxLevel: false },
  { level: 4,  requiredCaptures: 45,  requiredExp: 450,  maxStamina: 162, rewardGold: 200,  shopBonus: 6,   isMaxLevel: false },
  { level: 5,  requiredCaptures: 70,  requiredExp: 700,  maxStamina: 176, rewardGold: 300,  shopBonus: 8,   isMaxLevel: false },
  { level: 6,  requiredCaptures: 100, requiredExp: 1000, maxStamina: 190, rewardGold: 400,  shopBonus: 10,  isMaxLevel: false },
  { level: 7,  requiredCaptures: 140, requiredExp: 1400, maxStamina: 204, rewardGold: 500,  shopBonus: 12,  isMaxLevel: false },
  { level: 8,  requiredCaptures: 190, requiredExp: 1900, maxStamina: 218, rewardGold: 600,  shopBonus: 14,  isMaxLevel: false },
  { level: 9,  requiredCaptures: 250, requiredExp: 2500, maxStamina: 232, rewardGold: 700,  shopBonus: 16,  isMaxLevel: false },
  { level: 10, requiredCaptures: 320, requiredExp: 3200, maxStamina: 240, rewardGold: 1000, shopBonus: 20,  isMaxLevel: true  },
]

/** 每小时恢复体力点数 */
export const STAMINA_RECOVERY_PER_HOUR = 10

/** 自然恢复检查间隔（毫秒），每分钟检查一次 */
export const RECOVERY_TICK_MS = 60_000

/** 单次捕获体力消耗 */
export const CAPTURE_STAMINA_COST = 20

/** 单次派遣淘金币体力消耗 */
export const DISPATCH_STAMINA_COST = 20

/** 体力药剂恢复量 */
export const POTION_RECOVERY = 3

/** 体力药剂价格（金币） */
export const POTION_PRICE = 150

/** 每日限购次数 */
export const POTION_DAILY_LIMIT = 3

/** localStorage 存储 key */
export const STORAGE_KEY = 'animal_poke_stamina'

/** 每恢复 1 点体力所需的秒数 */
export const RECOVERY_SECONDS_PER_POINT = 360

/** 满级等级 */
export const MAX_LEVEL = 10

/** 捕获经验值表（按稀有度） */
export const CAPTURE_XP: Record<RarityTier, number> = {
  common: 8,
  uncommon: 15,
  rare: 30,
  epic: 60,
  legendary: 120,
}

/** 战斗胜利经验值 */
export const BATTLE_WIN_XP = 20

/** 战斗失败经验值 */
export const BATTLE_LOSE_XP = 5

/** 战斗平局经验值 */
export const BATTLE_DRAW_XP = 10

/** 战斗胜利稀有度额外加成 */
export const BATTLE_WIN_RARITY_BONUS: Record<RarityTier, number> = {
  common: 0,
  uncommon: 5,
  rare: 10,
  epic: 15,
  legendary: 20,
}

/** 每日签到经验值 */
export const CHECK_IN_XP = 15

/** 派遣完成经验值 */
export const DISPATCH_XP = 10
