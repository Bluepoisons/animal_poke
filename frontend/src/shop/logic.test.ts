import { describe, it, expect } from 'vitest'
import {
  getItemDef,
  canBuyItem,
  buyItem,
  getCheckInReward,
  calculateReward,
  canCheckIn,
  checkStreakBreak,
  getCheckInRewardForDay,
  getStreakInfo,
  calculateCaptureBoost,
  getTodayString,
  getYesterdayString,
  shouldResetDailyPurchases,
} from './logic'
import { ITEM_DEFS, CHECK_IN_REWARDS, CHECK_IN_EXP_REWARDS, CHECK_IN_CYCLE_DAYS } from './constants'
import type { CheckInState } from './types'

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
  it('#15 首次签到 streak=0 => newStreak=1, reward=20', () => {
    const result = getCheckInReward(0, '', BASE_TIME)
    expect(result.success).toBe(true)
    expect(result.canCheckIn).toBe(true)
    expect(result.newStreak).toBe(1)
    expect(result.reward).toBe(20)
    expect(result.rewardItem).toBeUndefined()
  })

  it('#16 连续第 3 天签到 => newStreak=3, reward=40', () => {
    const yesterday = getYesterdayString(BASE_TIME)
    const result = getCheckInReward(2, yesterday, BASE_TIME)
    expect(result.success).toBe(true)
    expect(result.newStreak).toBe(3)
    expect(result.reward).toBe(40)
  })

  it('#17 第 7 天满签 => newStreak=7, reward=150, rewardItem=toy_ball', () => {
    const yesterday = getYesterdayString(BASE_TIME)
    const result = getCheckInReward(6, yesterday, BASE_TIME)
    expect(result.success).toBe(true)
    expect(result.newStreak).toBe(7)
    expect(result.reward).toBe(150)
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

  it('#19 断签后重新签到 => newStreak=1, reward=20', () => {
    // 上次签到是 3 天前，不是昨天
    const result = getCheckInReward(5, '2025-01-12', BASE_TIME)
    expect(result.success).toBe(true)
    expect(result.newStreak).toBe(1)
    expect(result.reward).toBe(20)
  })
})

describe('getCheckInReward — 周期重置', () => {
  it('#20 满 7 天后第 8 天签到 => 周期重置为 newStreak=1, reward=20', () => {
    const yesterday = getYesterdayString(BASE_TIME)
    const result = getCheckInReward(7, yesterday, BASE_TIME)
    expect(result.success).toBe(true)
    expect(result.newStreak).toBe(1)
    expect(result.reward).toBe(20)
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

// ===== Issue #42: 连续签到 + 断签处理 增强测试 =====

describe('canCheckIn — 判断今日是否可签到', () => {
  it('#28 从未签到 lastCheckInDate="" => true', () => {
    expect(canCheckIn('', BASE_TIME)).toBe(true)
  })

  it('#29 今日已签到 lastCheckInDate=today => false', () => {
    const today = getTodayString(BASE_TIME)
    expect(canCheckIn(today, BASE_TIME)).toBe(false)
  })

  it('#30 昨日签到，今日未签 => true', () => {
    const yesterday = getYesterdayString(BASE_TIME)
    expect(canCheckIn(yesterday, BASE_TIME)).toBe(true)
  })
})

describe('checkStreakBreak — 断签判断', () => {
  it('#31 从未签到 => isBroken=false', () => {
    const result = checkStreakBreak('', BASE_TIME)
    expect(result.isBroken).toBe(false)
    expect(result.breakDate).toBe('')
  })

  it('#32 上次签到是昨天 => isBroken=false', () => {
    const yesterday = getYesterdayString(BASE_TIME)
    const result = checkStreakBreak(yesterday, BASE_TIME)
    expect(result.isBroken).toBe(false)
  })

  it('#33 上次签到是3天前 => isBroken=true, breakDate=today', () => {
    const today = getTodayString(BASE_TIME)
    const result = checkStreakBreak('2025-01-12', BASE_TIME)
    expect(result.isBroken).toBe(true)
    expect(result.breakDate).toBe(today)
  })

  it('#34 今日已签到 => isBroken=false', () => {
    const today = getTodayString(BASE_TIME)
    const result = checkStreakBreak(today, BASE_TIME)
    expect(result.isBroken).toBe(false)
  })
})

describe('calculateReward — 奖励计算（对齐设计文档数值）', () => {
  it('#35 首次签到 streak=0 => newStreak=1, reward=20, rewardExp=15', () => {
    const result = calculateReward(0, '', BASE_TIME)
    expect(result.success).toBe(true)
    expect(result.canCheckIn).toBe(true)
    expect(result.newStreak).toBe(1)
    expect(result.reward).toBe(20)
    expect(result.rewardExp).toBe(15)
    expect(result.wasReset).toBe(false)
    expect(result.rewardItem).toBeUndefined()
  })

  it('#36 连续第3天 streak=2, last=yesterday => newStreak=3, reward=40', () => {
    const yesterday = getYesterdayString(BASE_TIME)
    const result = calculateReward(2, yesterday, BASE_TIME)
    expect(result.success).toBe(true)
    expect(result.newStreak).toBe(3)
    expect(result.reward).toBe(40)
    expect(result.rewardExp).toBe(15)
    expect(result.wasReset).toBe(false)
  })

  it('#37 第7天满签 streak=6, last=yesterday => newStreak=7, reward=150, rewardExp=30, rewardItem=toy_ball', () => {
    const yesterday = getYesterdayString(BASE_TIME)
    const result = calculateReward(6, yesterday, BASE_TIME)
    expect(result.success).toBe(true)
    expect(result.newStreak).toBe(7)
    expect(result.reward).toBe(150)
    expect(result.rewardExp).toBe(30)
    expect(result.rewardItem).toBe('toy_ball')
    expect(result.wasReset).toBe(false)
  })

  it('#38 满7天后第8天 streak=7, last=yesterday => 周期重置 newStreak=1, reward=20, wasReset=false', () => {
    const yesterday = getYesterdayString(BASE_TIME)
    const result = calculateReward(7, yesterday, BASE_TIME)
    expect(result.success).toBe(true)
    expect(result.newStreak).toBe(1)
    expect(result.reward).toBe(20)
    expect(result.rewardExp).toBe(15)
    // 周期重置不算断签（wasReset 仅表示断签导致的重置）
    expect(result.wasReset).toBe(false)
  })
})

describe('calculateReward — 断签处理', () => {
  it('#39 断签后重新签到 streak=5, last=3天前 => newStreak=1, wasReset=true, reward=20', () => {
    const result = calculateReward(5, '2025-01-12', BASE_TIME)
    expect(result.success).toBe(true)
    expect(result.newStreak).toBe(1)
    expect(result.reward).toBe(20)
    expect(result.wasReset).toBe(true)
  })

  it('#40 今日已签到 => success=false, reason=already_checked_in', () => {
    const today = getTodayString(BASE_TIME)
    const result = calculateReward(3, today, BASE_TIME)
    expect(result.success).toBe(false)
    expect(result.canCheckIn).toBe(false)
    expect(result.reason).toBe('already_checked_in')
    expect(result.wasReset).toBe(false)
  })

  it('#41 断签后周期重置：streak=7, last=5天前 => newStreak=1, wasReset=true', () => {
    const result = calculateReward(7, '2025-01-10', BASE_TIME)
    expect(result.success).toBe(true)
    expect(result.newStreak).toBe(1)
    expect(result.wasReset).toBe(true)
  })
})

describe('getCheckInRewardForDay — 奖励定义查询', () => {
  it('#42 D1 => gold=20, exp=15, isMilestone=false', () => {
    const reward = getCheckInRewardForDay(1)
    expect(reward.day).toBe(1)
    expect(reward.gold).toBe(20)
    expect(reward.exp).toBe(15)
    expect(reward.isMilestone).toBe(false)
    expect(reward.bonusItem).toBeUndefined()
  })

  it('#43 D7 => gold=150, exp=30, isMilestone=true, bonusItem=toy_ball', () => {
    const reward = getCheckInRewardForDay(7)
    expect(reward.day).toBe(7)
    expect(reward.gold).toBe(150)
    expect(reward.exp).toBe(30)
    expect(reward.isMilestone).toBe(true)
    expect(reward.bonusItem).toBe('toy_ball')
  })

  it('#44 D0 越界 => clamp 到 D1', () => {
    const reward = getCheckInRewardForDay(0)
    expect(reward.day).toBe(1)
  })

  it('#45 D8 越界 => clamp 到 D7', () => {
    const reward = getCheckInRewardForDay(8)
    expect(reward.day).toBe(7)
  })
})

describe('getStreakInfo — 面板状态快照', () => {
  it('#46 未签到状态 streak=0 => hasCheckedInToday=false, todayCycleDay=1, isStreakBroken=false', () => {
    const state: CheckInState = {
      streak: 0, lastCheckInDate: '',
      totalCheckIns: 0, maxStreak: 0, lastBreakDate: '',
    }
    const info = getStreakInfo(state, BASE_TIME)
    expect(info.hasCheckedInToday).toBe(false)
    expect(info.todayCycleDay).toBe(1)
    expect(info.isStreakBroken).toBe(false)
    expect(info.todayReward.gold).toBe(20)
  })

  it('#47 连签3天且今日未签 => nextStreak=4, todayCycleDay=4, completedDays=[1,2,3]', () => {
    const yesterday = getYesterdayString(BASE_TIME)
    const state: CheckInState = {
      streak: 3, lastCheckInDate: yesterday,
      totalCheckIns: 10, maxStreak: 5, lastBreakDate: '',
    }
    const info = getStreakInfo(state, BASE_TIME)
    expect(info.hasCheckedInToday).toBe(false)
    expect(info.nextStreak).toBe(4)
    expect(info.todayCycleDay).toBe(4)
    expect(info.completedDays).toEqual([1, 2, 3])
    expect(info.isStreakBroken).toBe(false)
  })

  it('#48 断签状态 streak=5, last=3天前 => isStreakBroken=true, todayCycleDay=1', () => {
    const state: CheckInState = {
      streak: 5, lastCheckInDate: '2025-01-12',
      totalCheckIns: 20, maxStreak: 7, lastBreakDate: '',
    }
    const info = getStreakInfo(state, BASE_TIME)
    expect(info.isStreakBroken).toBe(true)
    expect(info.todayCycleDay).toBe(1)
    expect(info.todayReward.gold).toBe(20)
  })

  it('#49 今日已签到 streak=3 => hasCheckedInToday=true, todayCycleDay=3', () => {
    const today = getTodayString(BASE_TIME)
    const state: CheckInState = {
      streak: 3, lastCheckInDate: today,
      totalCheckIns: 3, maxStreak: 3, lastBreakDate: '',
    }
    const info = getStreakInfo(state, BASE_TIME)
    expect(info.hasCheckedInToday).toBe(true)
    expect(info.todayCycleDay).toBe(3)
    expect(info.nextStreak).toBe(3)
  })

  it('#50 满7天周期循环：streak=7, last=yesterday => todayCycleDay=1（新周期）', () => {
    const yesterday = getYesterdayString(BASE_TIME)
    const state: CheckInState = {
      streak: 7, lastCheckInDate: yesterday,
      totalCheckIns: 14, maxStreak: 7, lastBreakDate: '',
    }
    const info = getStreakInfo(state, BASE_TIME)
    expect(info.todayCycleDay).toBe(1)
    expect(info.isStreakBroken).toBe(false)
    expect(info.todayReward.gold).toBe(20)
  })
})

describe('奖励表完整性校验', () => {
  it('#51 CHECK_IN_REWARDS 长度为 7 且严格递增', () => {
    expect(CHECK_IN_REWARDS).toHaveLength(7)
    for (let i = 1; i < CHECK_IN_REWARDS.length; i++) {
      expect(CHECK_IN_REWARDS[i]).toBeGreaterThan(CHECK_IN_REWARDS[i - 1])
    }
  })

  it('#52 CHECK_IN_EXP_REWARDS 长度为 7 且第 7 天 > 其他天', () => {
    expect(CHECK_IN_EXP_REWARDS).toHaveLength(7)
    const day7Exp = CHECK_IN_EXP_REWARDS[6]
    for (let i = 0; i < 6; i++) {
      expect(day7Exp).toBeGreaterThan(CHECK_IN_EXP_REWARDS[i])
    }
  })

  it('#53 D7 金币奖励（150）对齐设计文档', () => {
    expect(CHECK_IN_REWARDS[6]).toBe(150)
  })
})
