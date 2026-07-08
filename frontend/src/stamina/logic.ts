import {
  LEVEL_TABLE,
  POTION_PRICE,
  POTION_DAILY_LIMIT,
  RECOVERY_SECONDS_PER_POINT,
} from './constants'
import type { RecoveryResult, LevelUpResult, BuyPotionResult } from './types'

/** 根据等级获取体力上限，level 越界时 clamp 到 1~10 */
export function getMaxStamina(level: number): number {
  const clampedLevel = Math.max(1, Math.min(10, level))
  return LEVEL_TABLE[clampedLevel - 1].maxStamina
}

/** 根据累计捕获数计算应处等级（1~10） */
export function getLevelForCaptures(totalCaptures: number): number {
  // 从最高等级往下找，找到第一个 requiredCaptures <= totalCaptures 的
  for (let i = LEVEL_TABLE.length - 1; i >= 0; i--) {
    if (totalCaptures >= LEVEL_TABLE[i].requiredCaptures) {
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
    // 体力已满时 recoverTime = 0，否则 = 360
    return {
      current: currentStamina,
      recoverTime: currentStamina >= maxStamina ? 0 : RECOVERY_SECONDS_PER_POINT,
    }
  }
  const elapsedSec = elapsedMs / 1000
  const recovered = Math.floor(elapsedSec / RECOVERY_SECONDS_PER_POINT)
  const newStamina = Math.min(currentStamina + recovered, maxStamina)

  // 体力已满时 recoverTime = 0，否则计算距下次恢复的秒数
  const recoverTime = newStamina >= maxStamina
    ? 0
    : RECOVERY_SECONDS_PER_POINT - Math.floor(elapsedSec % RECOVERY_SECONDS_PER_POINT)

  return { current: newStamina, recoverTime }
}

/**
 * 检查并执行升级
 * 从当前等级+1 开始遍历等级表，累计所有可跨越等级的奖励金币
 */
export function tryLevelUp(currentLevel: number, totalCaptures: number): LevelUpResult {
  if (currentLevel >= 10) {
    return { leveledUp: false, newLevel: currentLevel, rewardGold: 0 }
  }

  let newLevel = currentLevel
  let rewardGold = 0

  // 从当前等级+1 开始检查，看能升到哪级
  for (let i = currentLevel; i < LEVEL_TABLE.length; i++) {
    const entry = LEVEL_TABLE[i]
    if (totalCaptures >= entry.requiredCaptures) {
      newLevel = entry.level
      rewardGold += entry.rewardGold
    }
  }

  if (newLevel > currentLevel) {
    return { leveledUp: true, newLevel, rewardGold }
  }

  return { leveledUp: false, newLevel: currentLevel, rewardGold: 0 }
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
