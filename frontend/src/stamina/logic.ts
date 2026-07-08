import type { RarityTier } from '../types'
import {
  LEVEL_TABLE,
  POTION_PRICE,
  POTION_DAILY_LIMIT,
  RECOVERY_SECONDS_PER_POINT,
  MAX_LEVEL,
  CAPTURE_XP,
  BATTLE_WIN_XP,
  BATTLE_LOSE_XP,
  BATTLE_DRAW_XP,
  BATTLE_WIN_RARITY_BONUS,
} from './constants'
import type { RecoveryResult, LevelUpResult, BuyPotionResult, StaminaState } from './types'

/** 根据等级获取体力上限，level 越界时 clamp 到 1~10 */
export function getMaxStamina(level: number): number {
  const clampedLevel = Math.max(1, Math.min(10, level))
  return LEVEL_TABLE[clampedLevel - 1].maxStamina
}

/** 根据累计捕获数计算应处等级（1~10），保留兼容旧逻辑 */
export function getLevelForCaptures(totalCaptures: number): number {
  for (let i = LEVEL_TABLE.length - 1; i >= 0; i--) {
    if (totalCaptures >= LEVEL_TABLE[i].requiredCaptures) {
      return LEVEL_TABLE[i].level
    }
  }
  return 1
}

/** 根据经验值计算应处等级（1~10） */
export function getLevelForExp(exp: number): number {
  for (let i = LEVEL_TABLE.length - 1; i >= 0; i--) {
    if (exp >= LEVEL_TABLE[i].requiredExp) {
      return LEVEL_TABLE[i].level
    }
  }
  return 1
}

/**
 * 计算自然恢复后的体力值
 * 基于 lastRecoverTime 与 now 的时间差计算恢复量
 * 每 360 秒恢复 1 点，支持离线恢复
 */
export function calculateRecovery(
  lastRecoverTime: number,
  currentStamina: number,
  maxStamina: number,
  now?: number
): RecoveryResult {
  const currentTime = now ?? Date.now()
  const elapsedMs = currentTime - lastRecoverTime
  // elapsed < 0 说明时间异常（时钟回拨），不做恢复
  if (elapsedMs < 0) {
    return {
      current: currentStamina,
      recoverTime: currentStamina >= maxStamina ? 0 : RECOVERY_SECONDS_PER_POINT,
    }
  }
  const elapsedSec = elapsedMs / 1000
  const recovered = Math.floor(elapsedSec / RECOVERY_SECONDS_PER_POINT)
  const newStamina = Math.min(currentStamina + recovered, maxStamina)

  const recoverTime = newStamina >= maxStamina
    ? 0
    : RECOVERY_SECONDS_PER_POINT - Math.floor(elapsedSec % RECOVERY_SECONDS_PER_POINT)

  return { current: newStamina, recoverTime }
}

/**
 * 检查并执行升级（exp 驱动）
 * 从当前等级+1 开始遍历等级表，累计所有可跨越等级的奖励金币
 */
export function tryLevelUp(currentLevel: number, exp: number): LevelUpResult {
  if (currentLevel >= MAX_LEVEL) {
    return { leveledUp: false, newLevel: currentLevel, rewardGold: 0, crossedLevels: [] }
  }

  let newLevel = currentLevel
  let rewardGold = 0
  const crossedLevels: number[] = []

  for (let i = currentLevel; i < LEVEL_TABLE.length; i++) {
    const entry = LEVEL_TABLE[i]
    if (exp >= entry.requiredExp) {
      newLevel = entry.level
      rewardGold += entry.rewardGold
      crossedLevels.push(entry.level)
    }
  }

  if (newLevel > currentLevel) {
    return { leveledUp: true, newLevel, rewardGold, crossedLevels }
  }

  return { leveledUp: false, newLevel: currentLevel, rewardGold: 0, crossedLevels: [] }
}

/**
 * 获取当前等级的经验值信息（用于 UI 进度条）
 */
export function getExpProgress(level: number, exp: number): {
  currentLevelExp: number
  nextLevelExp: number
  progress: number
} {
  const currentEntry = LEVEL_TABLE[Math.max(0, Math.min(LEVEL_TABLE.length - 1, level - 1))]
  const currentLevelExpBase = currentEntry.requiredExp

  if (level >= MAX_LEVEL) {
    return { currentLevelExp: exp - currentLevelExpBase, nextLevelExp: 0, progress: 100 }
  }

  const nextEntry = LEVEL_TABLE[level]
  const nextLevelExpTotal = nextEntry.requiredExp - currentEntry.requiredExp
  const currentLevelExp = exp - currentLevelExpBase
  const progress = nextLevelExpTotal > 0
    ? Math.min(100, Math.round((currentLevelExp / nextLevelExpTotal) * 100))
    : 0

  return { currentLevelExp, nextLevelExp: nextLevelExpTotal, progress }
}

/** 获取指定稀有度的捕获经验值 */
export function getCaptureXp(rarity: RarityTier): number {
  return CAPTURE_XP[rarity]
}

/** 计算战斗经验值 */
export function getBattleXp(result: 'win' | 'lose' | 'draw', enemyRarity: RarityTier): number {
  if (result === 'win') return BATTLE_WIN_XP + BATTLE_WIN_RARITY_BONUS[enemyRarity]
  if (result === 'draw') return BATTLE_DRAW_XP
  return BATTLE_LOSE_XP
}

/** 检查体力是否足够消耗 */
export function canConsume(currentStamina: number, amount: number): boolean {
  return currentStamina >= amount
}

/** 获取今日日期标记（自然日，格式 'YYYY-MM-DD'） */
export function getTodayString(now?: number): string {
  const date = now ? new Date(now) : new Date()
  const y = date.getFullYear()
  const m = String(date.getMonth() + 1).padStart(2, '0')
  const d = String(date.getDate()).padStart(2, '0')
  return `${y}-${m}-${d}`
}

/** 检查购买日期是否需要重置（不是今天则需重置） */
export function shouldResetDailyPurchases(potionPurchaseDate: string, now?: number): boolean {
  return potionPurchaseDate !== getTodayString(now)
}

/** 计算购买体力药剂的结果（纯函数，不修改状态） */
export function calculateBuyPotion(gold: number, potionPurchasesToday: number): BuyPotionResult {
  if (potionPurchasesToday >= POTION_DAILY_LIMIT) {
    return { success: false, remainingPurchases: 0, reason: 'daily_limit_reached' }
  }
  if (gold < POTION_PRICE) {
    return { success: false, remainingPurchases: POTION_DAILY_LIMIT - potionPurchasesToday, reason: 'insufficient_gold' }
  }
  return { success: true, remainingPurchases: POTION_DAILY_LIMIT - potionPurchasesToday - 1 }
}

/**
 * 旧存档数据迁移：若无 exp 字段，以 totalCaptures × 10 推算
 */
export function migrateState(saved: Partial<StaminaState>): StaminaState {
  const defaultState: StaminaState = {
    level: 1,
    exp: 0,
    currentStamina: 120,
    totalCaptures: 0,
    lastRecoverTime: Date.now(),
    gold: 0,
    potionPurchasesToday: 0,
    potionPurchaseDate: getTodayString(),
    totalBattlesWon: 0,
    totalBattles: 0,
    currentWinStreak: 0,
    maxWinStreak: 0,
  }
  const merged = { ...defaultState, ...saved }
  // 如果没有 exp 字段但有 totalCaptures，推算 exp
  if (typeof saved.exp !== 'number' && typeof saved.totalCaptures === 'number') {
    merged.exp = saved.totalCaptures * 10
  }
  // 确保 level 与 exp 一致
  merged.level = getLevelForExp(merged.exp)
  return merged
}
