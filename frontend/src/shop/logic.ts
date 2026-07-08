import {
  ITEM_DEFS,
  CHECK_IN_REWARDS,
  CHECK_IN_EXP_REWARDS,
  CHECK_IN_CYCLE_DAYS,
  CHECK_IN_DAY7_BONUS_ITEM,
} from './constants'
import type { ItemId, ItemDef } from './constants'
import type { Inventory, BuyResult, CheckInResult, CheckInReward, CheckInStatus, CheckInState } from './types'

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
 * 判断今日是否可签到
 * @param lastCheckInDate 上次签到日期 'YYYY-MM-DD'
 * @param now 当前时间戳（可选，测试注入）
 * @returns true = 今日尚未签到
 */
export function canCheckIn(lastCheckInDate: string, now?: number): boolean {
  const today = getTodayString(now)
  return lastCheckInDate !== today
}

/**
 * 判断签到是否断签
 * 断签定义：上次签到日期不是昨天且不是今天（即间隔 ≥ 2 天）
 * @param lastCheckInDate 上次签到日期 'YYYY-MM-DD'
 * @param now 当前时间戳（可选，测试注入）
 * @returns { isBroken: 是否断签, breakDate: 断签发生的日期 }
 */
export function checkStreakBreak(
  lastCheckInDate: string,
  now?: number
): { isBroken: boolean; breakDate: string } {
  if (!lastCheckInDate) {
    // 从未签到过，不算断签
    return { isBroken: false, breakDate: '' }
  }

  const today = getTodayString(now)
  const yesterday = getYesterdayString(now)

  // 今天已签到 → 未断签
  if (lastCheckInDate === today) {
    return { isBroken: false, breakDate: '' }
  }

  // 上次签到是昨天 → 未断签
  if (lastCheckInDate === yesterday) {
    return { isBroken: false, breakDate: '' }
  }

  // 上次签到既不是昨天也不是今天 → 断签
  return { isBroken: true, breakDate: today }
}

/**
 * 计算签到结果（纯函数，不修改状态）
 *
 * 逻辑流程：
 * 1. 今日已签到 → 返回失败
 * 2. 判断是否断签 → 断签则重置为第 1 天
 * 3. 未断签 → streak + 1
 * 4. 周期处理：streak > 7 → 重置为第 1 天（新周期）
 * 5. 根据周期天数查奖励表
 * 6. 第 7 天额外送道具
 *
 * @param streak 当前连续签到天数
 * @param lastCheckInDate 上次签到日期 'YYYY-MM-DD'
 * @param now 当前时间戳（可选，测试注入）
 */
export function calculateReward(
  streak: number,
  lastCheckInDate: string,
  now?: number
): CheckInResult {
  const today = getTodayString(now)

  // 1. 今日已签到
  if (!canCheckIn(lastCheckInDate, now)) {
    return {
      success: false,
      canCheckIn: false,
      newStreak: streak,
      reward: 0,
      rewardExp: 0,
      wasReset: false,
      reason: 'already_checked_in',
    }
  }

  // 2. 判断是否断签
  const breakInfo = checkStreakBreak(lastCheckInDate, now)
  const wasReset = breakInfo.isBroken

  // 3. 计算新 streak
  let newStreak: number
  if (wasReset) {
    // 断签 → 重置为第 1 天
    newStreak = 1
  } else {
    // 未断签 → streak + 1
    newStreak = streak + 1
  }

  // 4. 周期处理：满 7 天后重置周期为第 1 天
  if (newStreak > CHECK_IN_CYCLE_DAYS) {
    newStreak = 1
  }

  // 5. 获取奖励（基于周期天数，索引 0~6）
  const rewardIndex = newStreak - 1
  const reward = CHECK_IN_REWARDS[rewardIndex]
  const rewardExp = CHECK_IN_EXP_REWARDS[rewardIndex]

  // 6. 第 7 天额外送道具
  const rewardItem = newStreak === CHECK_IN_CYCLE_DAYS
    ? CHECK_IN_DAY7_BONUS_ITEM
    : undefined

  return {
    success: true,
    canCheckIn: true,
    newStreak,
    reward,
    rewardExp,
    wasReset,
    rewardItem,
  }
}

/**
 * 获取指定天数的签到奖励定义（用于 UI 展示）
 * @param day 天数 1~7
 */
export function getCheckInRewardForDay(day: number): CheckInReward {
  const index = Math.max(0, Math.min(CHECK_IN_CYCLE_DAYS - 1, day - 1))
  return {
    day: index + 1,
    gold: CHECK_IN_REWARDS[index],
    exp: CHECK_IN_EXP_REWARDS[index],
    bonusItem: index === CHECK_IN_CYCLE_DAYS - 1 ? CHECK_IN_DAY7_BONUS_ITEM : undefined,
    isMilestone: index === CHECK_IN_CYCLE_DAYS - 1,
  }
}

/**
 * 获取签到面板状态快照（供 UI 渲染用）
 * @param state 当前签到状态
 * @param now 当前时间戳（可选，测试注入）
 */
export function getStreakInfo(
  state: CheckInState,
  now?: number
): CheckInStatus {
  const today = getTodayString(now)
  const hasCheckedInToday = state.lastCheckInDate === today

  const breakInfo = checkStreakBreak(state.lastCheckInDate, now)

  // 今日是本周期第几天
  let todayCycleDay: number
  if (hasCheckedInToday) {
    todayCycleDay = state.streak
  } else if (breakInfo.isBroken) {
    todayCycleDay = 1 // 断签后从第 1 天开始
  } else {
    todayCycleDay = state.streak + 1
    if (todayCycleDay > CHECK_IN_CYCLE_DAYS) {
      todayCycleDay = 1 // 周期循环
    }
  }

  // 已完成签到的天数列表
  const completedDays: number[] = []
  if (state.streak > 0 && !breakInfo.isBroken) {
    for (let i = 1; i <= state.streak; i++) {
      completedDays.push(i)
    }
  }
  // 今日已签到则也加入已完成
  if (hasCheckedInToday && !completedDays.includes(state.streak)) {
    completedDays.push(state.streak)
  }

  // 今日可获得奖励
  const todayReward = getCheckInRewardForDay(todayCycleDay)

  // 明日可获得奖励（预览）
  const tomorrowCycleDay = todayCycleDay >= CHECK_IN_CYCLE_DAYS ? 1 : todayCycleDay + 1
  const tomorrowReward = getCheckInRewardForDay(tomorrowCycleDay)

  // nextStreak：今日签到后将达到的天数
  const nextStreak = hasCheckedInToday ? state.streak : todayCycleDay

  return {
    hasCheckedInToday,
    currentStreak: state.streak,
    nextStreak,
    completedDays,
    todayCycleDay,
    isStreakBroken: breakInfo.isBroken && !hasCheckedInToday,
    todayReward,
    tomorrowReward,
    maxStreak: state.maxStreak,
    totalCheckIns: state.totalCheckIns,
  }
}

/**
 * @deprecated 使用 calculateReward 替代
 * 兼容旧调用方的别名
 */
export function getCheckInReward(
  streak: number,
  lastCheckInDate: string,
  now?: number
): CheckInResult {
  return calculateReward(streak, lastCheckInDate, now)
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
