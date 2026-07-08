import {
  ITEM_DEFS,
  CHECK_IN_REWARDS,
  CHECK_IN_CYCLE_DAYS,
  CHECK_IN_DAY7_BONUS_ITEM,
} from './constants'
import type { ItemId, ItemDef } from './constants'
import type { Inventory, BuyResult, CheckInResult } from './types'

/** 获取道具定义 */
export function getItemDef(itemId: ItemId): ItemDef {
  return ITEM_DEFS[itemId]
}

/** 获取今日日期标记（自然日，格式 'YYYY-MM-DD'） */
export function getTodayString(now?: number): string {
  const date = now ? new Date(now) : new Date()
  const y = date.getFullYear()
  const m = String(date.getMonth() + 1).padStart(2, '0')
  const d = String(date.getDate()).padStart(2, '0')
  return `${y}-${m}-${d}`
}

/** 获取昨日日期标记 */
export function getYesterdayString(now?: number): string {
  const date = now ? new Date(now) : new Date()
  date.setDate(date.getDate() - 1)
  const y = date.getFullYear()
  const m = String(date.getMonth() + 1).padStart(2, '0')
  const d = String(date.getDate()).padStart(2, '0')
  return `${y}-${m}-${d}`
}

/** 检查每日限购是否需要重置（日期不是今天则需重置） */
export function shouldResetDailyPurchases(date: string, now?: number): boolean {
  return date !== getTodayString(now)
}

/** 检查金币是否足够购买 */
export function canBuyItem(gold: number, price: number): boolean {
  return gold >= price
}

/**
 * 计算购买道具结果（纯函数，不修改状态）
 * @param gold 当前金币
 * @param price 道具价格
 * @param dailyPurchased 今日已购买次数
 * @param dailyLimit 每日限购次数（0 表示不限购）
 */
export function buyItem(
  gold: number,
  price: number,
  dailyPurchased: number = 0,
  dailyLimit: number = 0
): BuyResult {
  // 检查每日限购
  if (dailyLimit > 0 && dailyPurchased >= dailyLimit) {
    return {
      success: false,
      reason: 'daily_limit_reached',
      remainingGold: gold,
      remainingDailyPurchases: 0,
    }
  }
  // 检查金币
  if (!canBuyItem(gold, price)) {
    return {
      success: false,
      reason: 'insufficient_gold',
      remainingGold: gold,
      remainingDailyPurchases: dailyLimit > 0 ? dailyLimit - dailyPurchased : null,
    }
  }
  return {
    success: true,
    remainingGold: gold - price,
    remainingDailyPurchases: dailyLimit > 0 ? dailyLimit - dailyPurchased - 1 : null,
  }
}

/**
 * 计算签到结果（纯函数）
 * @param streak 当前连续签到天数
 * @param lastCheckInDate 上次签到日期 'YYYY-MM-DD'
 * @param now 当前时间戳（可选，用于测试注入）
 */
export function getCheckInReward(
  streak: number,
  lastCheckInDate: string,
  now?: number
): CheckInResult {
  const today = getTodayString(now)

  // 今日已签到
  if (lastCheckInDate === today) {
    return {
      success: false,
      canCheckIn: false,
      newStreak: streak,
      reward: 0,
      reason: 'already_checked_in',
    }
  }

  // 判断是否断签：上次签到不是昨天则断签
  const yesterday = getYesterdayString(now)
  const isBroken = lastCheckInDate !== yesterday

  // 断签重置为第 1 天
  const newStreak = isBroken ? 1 : streak + 1

  // 周期处理：满 7 天后重置周期为第 1 天
  const cycleDay = newStreak > CHECK_IN_CYCLE_DAYS ? 1 : newStreak

  // 获取奖励（基于周期天数，索引 0~6）
  const reward = CHECK_IN_REWARDS[cycleDay - 1]
  const rewardItem = cycleDay === CHECK_IN_CYCLE_DAYS ? CHECK_IN_DAY7_BONUS_ITEM : undefined

  return {
    success: true,
    canCheckIn: true,
    newStreak: cycleDay,
    reward,
    rewardItem,
  }
}

/**
 * 计算捕获增益百分比
 * @param activeBoost 已激活的道具 ID（null 表示无增益）
 */
export function calculateCaptureBoost(activeBoost: ItemId | null): number {
  if (!activeBoost) return 0
  return ITEM_DEFS[activeBoost]?.captureBoost ?? 0
}

/** 查询道具持有数量 */
export function getItemCount(inventory: Inventory, itemId: ItemId): number {
  return inventory[itemId] ?? 0
}
