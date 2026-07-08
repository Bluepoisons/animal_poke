import { describe, it, expect } from 'vitest'
import {
  getItemDef,
  canBuyItem,
  buyItem,
  getCheckInReward,
  calculateCaptureBoost,
  getTodayString,
  getYesterdayString,
  shouldResetDailyPurchases,
} from './logic'
import { ITEM_DEFS, CHECK_IN_REWARDS, CHECK_IN_CYCLE_DAYS } from './constants'

// 固定时间戳：2025-01-15 12:00:00 UTC（周三）
const BASE_TIME = 1736932800000

describe('getItemDef', () => {
  it('#1 获取玩具球定义：id/toy_ball, price/50, captureBoost/15', () => {
    const def = getItemDef('toy_ball')
    expect(def.id).toBe('toy_ball')
    expect(def.price).toBe(50)
    expect(def.captureBoost).toBe(15)
  })

  it('#2 获取体力药剂定义：dailyLimit/3, captureBoost/0', () => {
    const def = getItemDef('stamina_potion')
    expect(def.dailyLimit).toBe(3)
    expect(def.captureBoost).toBe(0)
  })
})

describe('canBuyItem', () => {
  it('#3 金币足够 => true', () => {
    expect(canBuyItem(100, 50)).toBe(true)
  })

  it('#4 金币不足 => false', () => {
    expect(canBuyItem(30, 50)).toBe(false)
  })

  it('#5 金币刚好等于价格 => true', () => {
    expect(canBuyItem(50, 50)).toBe(true)
  })
})

describe('buyItem — 正常购买', () => {
  it('#6 玩具球 gold=100, 无限购 => success, remainingGold=50', () => {
    const result = buyItem(100, 50, 0, 0)
    expect(result.success).toBe(true)
    expect(result.remainingGold).toBe(50)
    expect(result.remainingDailyPurchases).toBeNull()
  })

  it('#7 体力药剂 gold=200, dailyPurchased=0 => success, remainingDaily=2', () => {
    const result = buyItem(200, 150, 0, 3)
    expect(result.success).toBe(true)
    expect(result.remainingGold).toBe(50)
    expect(result.remainingDailyPurchases).toBe(2)
  })
})

describe('buyItem — 金币不足', () => {
  it('#8 gold=30 < price=50 => insufficient_gold', () => {
    const result = buyItem(30, 50, 0, 0)
    expect(result.success).toBe(false)
    expect(result.reason).toBe('insufficient_gold')
    expect(result.remainingGold).toBe(30)
  })

  it('#9 gold=0 购买食物包 => insufficient_gold', () => {
    const result = buyItem(0, 30, 0, 0)
    expect(result.success).toBe(false)
    expect(result.reason).toBe('insufficient_gold')
  })
})

describe('buyItem — 每日限购', () => {
  it('#10 体力药剂 dailyPurchased=3 => daily_limit_reached', () => {
    const result = buyItem(999, 150, 3, 3)
    expect(result.success).toBe(false)
    expect(result.reason).toBe('daily_limit_reached')
    expect(result.remainingDailyPurchases).toBe(0)
  })

  it('#11 体力药剂 dailyPurchased=2（最后一次）=> success, remainingDaily=0', () => {
    const result = buyItem(200, 150, 2, 3)
    expect(result.success).toBe(true)
    expect(result.remainingDailyPurchases).toBe(0)
  })

  it('#12 玩具球不限购，dailyPurchased=99 仍可购买', () => {
    const result = buyItem(100, 50, 99, 0)
    expect(result.success).toBe(true)
  })
})

describe('buyItem — 边界值', () => {
  it('#13 gold 刚好等于 price => success', () => {
    const result = buyItem(50, 50, 0, 0)
    expect(result.success).toBe(true)
    expect(result.remainingGold).toBe(0)
  })

  it('#14 gold=price-1 => insufficient_gold', () => {
    const result = buyItem(49, 50, 0, 0)
    expect(result.success).toBe(false)
    expect(result.reason).toBe('insufficient_gold')
  })
})

describe('getCheckInReward — 正常签到', () => {
  it('#15 首次签到 streak=0 => newStreak=1, reward=10', () => {
    const result = getCheckInReward(0, '', BASE_TIME)
    expect(result.success).toBe(true)
    expect(result.canCheckIn).toBe(true)
    expect(result.newStreak).toBe(1)
    expect(result.reward).toBe(10)
    expect(result.rewardItem).toBeUndefined()
  })

  it('#16 连续第 3 天签到 => newStreak=3, reward=30', () => {
    const yesterday = getYesterdayString(BASE_TIME)
    const result = getCheckInReward(2, yesterday, BASE_TIME)
    expect(result.success).toBe(true)
    expect(result.newStreak).toBe(3)
    expect(result.reward).toBe(30)
  })

  it('#17 第 7 天满签 => newStreak=7, reward=200, rewardItem=toy_ball', () => {
    const yesterday = getYesterdayString(BASE_TIME)
    const result = getCheckInReward(6, yesterday, BASE_TIME)
    expect(result.success).toBe(true)
    expect(result.newStreak).toBe(7)
    expect(result.reward).toBe(200)
    expect(result.rewardItem).toBe('toy_ball')
  })
})

describe('getCheckInReward — 断签与重复', () => {
  it('#18 今日已签到 => already_checked_in', () => {
    const today = getTodayString(BASE_TIME)
    const result = getCheckInReward(3, today, BASE_TIME)
    expect(result.success).toBe(false)
    expect(result.canCheckIn).toBe(false)
    expect(result.reason).toBe('already_checked_in')
  })

  it('#19 断签后重新签到 => newStreak=1, reward=10', () => {
    // 上次签到是 3 天前，不是昨天
    const result = getCheckInReward(5, '2025-01-12', BASE_TIME)
    expect(result.success).toBe(true)
    expect(result.newStreak).toBe(1)
    expect(result.reward).toBe(10)
  })
})

describe('getCheckInReward — 周期重置', () => {
  it('#20 满 7 天后第 8 天签到 => 周期重置为 newStreak=1, reward=10', () => {
    const yesterday = getYesterdayString(BASE_TIME)
    const result = getCheckInReward(7, yesterday, BASE_TIME)
    expect(result.success).toBe(true)
    expect(result.newStreak).toBe(1)
    expect(result.reward).toBe(10)
  })
})

describe('calculateCaptureBoost', () => {
  it('#21 无激活道具 => boost=0', () => {
    expect(calculateCaptureBoost(null)).toBe(0)
  })

  it('#22 激活玩具球 => boost=15', () => {
    expect(calculateCaptureBoost('toy_ball')).toBe(15)
  })

  it('#23 激活高级玩具球 => boost=25', () => {
    expect(calculateCaptureBoost('premium_toy_ball')).toBe(25)
  })

  it('#24 激活感冒药（非捕获道具）=> boost=0', () => {
    expect(calculateCaptureBoost('cold_medicine')).toBe(0)
  })
})

describe('shouldResetDailyPurchases', () => {
  it('#25 跨日重置：日期不是今天 => true', () => {
    const today = getTodayString(BASE_TIME)
    expect(shouldResetDailyPurchases('2025-01-14', BASE_TIME)).toBe(true)
    expect(shouldResetDailyPurchases(today, BASE_TIME)).toBe(false)
  })
})

describe('签到奖励表完整性', () => {
  it('#26 奖励表长度为 7 天', () => {
    expect(CHECK_IN_REWARDS).toHaveLength(CHECK_IN_CYCLE_DAYS)
  })

  it('#27 奖励递增', () => {
    for (let i = 1; i < CHECK_IN_REWARDS.length; i++) {
      expect(CHECK_IN_REWARDS[i]).toBeGreaterThan(CHECK_IN_REWARDS[i - 1])
    }
  })
})
