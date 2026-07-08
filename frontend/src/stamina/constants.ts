import type { LevelTableRow } from './types'

/** 等级表（索引 0 = Lv.1，索引 9 = Lv.10） */
export const LEVEL_TABLE: LevelTableRow[] = [
  { level: 1,  requiredCaptures: 0,   maxStamina: 120, rewardGold: 0,    isMaxLevel: false },
  { level: 2,  requiredCaptures: 10,  maxStamina: 134, rewardGold: 100,  isMaxLevel: false },
  { level: 3,  requiredCaptures: 25,  maxStamina: 148, rewardGold: 150,  isMaxLevel: false },
  { level: 4,  requiredCaptures: 45,  maxStamina: 162, rewardGold: 200,  isMaxLevel: false },
  { level: 5,  requiredCaptures: 70,  maxStamina: 176, rewardGold: 300,  isMaxLevel: false },
  { level: 6,  requiredCaptures: 100, maxStamina: 190, rewardGold: 400,  isMaxLevel: false },
  { level: 7,  requiredCaptures: 140, maxStamina: 204, rewardGold: 500,  isMaxLevel: false },
  { level: 8,  requiredCaptures: 190, maxStamina: 218, rewardGold: 600,  isMaxLevel: false },
  { level: 9,  requiredCaptures: 250, maxStamina: 232, rewardGold: 700,  isMaxLevel: false },
  { level: 10, requiredCaptures: 320, maxStamina: 240, rewardGold: 1000, isMaxLevel: true  },
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
