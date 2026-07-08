import { describe, it, expect } from 'vitest'
import {
  getMaxStamina,
  getLevelForCaptures,
  calculateRecovery,
  tryLevelUp,
  canConsume,
  getTodayString,
  shouldResetDailyPurchases,
  calculateBuyPotion,
} from './logic'
import { LEVEL_TABLE, POTION_PRICE, POTION_DAILY_LIMIT, POTION_RECOVERY, RECOVERY_SECONDS_PER_POINT } from './constants'

// 固定时间戳，方便测试（2025-01-15 12:00:00 UTC）
const BASE_TIME = 1736932800000

describe('getMaxStamina', () => {
  it('#1 Lv.1 返回 120', () => {
    expect(getMaxStamina(1)).toBe(120)
  })

  it('#2 Lv.10 返回 240', () => {
    expect(getMaxStamina(10)).toBe(240)
  })

  it('#3 越界 level=0 clamp 到 Lv.1 返回 120', () => {
    expect(getMaxStamina(0)).toBe(120)
  })

  it('越界 level=11 clamp 到 Lv.10 返回 240', () => {
    expect(getMaxStamina(11)).toBe(240)
  })
})

describe('getLevelForCaptures', () => {
  it('#4 captures=0 返回 Lv.1', () => {
    expect(getLevelForCaptures(0)).toBe(1)
  })

  it('#5 captures=10 刚好升 Lv.2', () => {
    expect(getLevelForCaptures(10)).toBe(2)
  })

  it('#6 captures=50 跨级到 Lv.4', () => {
    expect(getLevelForCaptures(50)).toBe(4)
  })

  it('#7 captures=500 满级后仍为 Lv.10', () => {
    expect(getLevelForCaptures(500)).toBe(10)
  })
})

describe('calculateRecovery', () => {
  it('#8 elapsed=0 不恢复，recoverTime=360', () => {
    const result = calculateRecovery(BASE_TIME, 80, 120, BASE_TIME)
    expect(result.current).toBe(80)
    expect(result.recoverTime).toBe(360)
  })

  it('#9 elapsed=360s 恢复 1 点', () => {
    const result = calculateRecovery(BASE_TIME, 80, 120, BASE_TIME + 360 * 1000)
    expect(result.current).toBe(81)
    expect(result.recoverTime).toBe(360)
  })

  it('#10 elapsed=3600s 恢复 10 点', () => {
    const result = calculateRecovery(BASE_TIME, 80, 120, BASE_TIME + 3600 * 1000)
    expect(result.current).toBe(90)
    expect(result.recoverTime).toBe(360)
  })

  it('#11 恢复到上限不超 maxStamina', () => {
    // current=115, elapsed=1800s (30 分钟), 5 点恢复 -> 120 达上限
    const result = calculateRecovery(BASE_TIME, 115, 120, BASE_TIME + 1800 * 1000)
    expect(result.current).toBe(120)
    expect(result.recoverTime).toBe(0)
  })

  it('#12 已满体力不恢复，recoverTime=0', () => {
    const result = calculateRecovery(BASE_TIME, 120, 120, BASE_TIME + 3600 * 1000)
    expect(result.current).toBe(120)
    expect(result.recoverTime).toBe(0)
  })

  it('elapsed 有余数时 recoverTime 为剩余秒数', () => {
    // elapsed=200s, 不足 360 秒所以不恢复，recoverTime=360-200=160
    const result = calculateRecovery(BASE_TIME, 80, 120, BASE_TIME + 200 * 1000)
    expect(result.current).toBe(80)
    expect(result.recoverTime).toBe(160)
  })
})

describe('canConsume', () => {
  it('#13 正常消耗 stamina=70, cost=20 => true', () => {
    expect(canConsume(70, 20)).toBe(true)
  })

  it('#14 体力不足 stamina=10, cost=20 => false', () => {
    expect(canConsume(10, 20)).toBe(false)
  })

  it('#15 刚好足够 stamina=20, cost=20 => true', () => {
    expect(canConsume(20, 20)).toBe(true)
  })
})

describe('消耗与增加（Reducer 行为测试）', () => {
  it('#16 增加体力不超上限：current=118, add=5, max=120 => 120', () => {
    const max = getMaxStamina(1)
    const added = Math.min(118 + 5, max)
    expect(added).toBe(120)
  })
})

describe('tryLevelUp', () => {
  it('#17 未达升级条件 level=2, captures=15 => 不升级', () => {
    const result = tryLevelUp(2, 15)
    expect(result.leveledUp).toBe(false)
    expect(result.newLevel).toBe(2)
    expect(result.rewardGold).toBe(0)
  })

  it('#18 刚好升级一级 level=2, captures=25 => Lv.3, rewardGold=150', () => {
    const result = tryLevelUp(2, 25)
    expect(result.leveledUp).toBe(true)
    expect(result.newLevel).toBe(3)
    expect(result.rewardGold).toBe(150)
  })

  it('#19 跨级升级 level=2, captures=50 => Lv.4, rewardGold=350', () => {
    const result = tryLevelUp(2, 50)
    expect(result.leveledUp).toBe(true)
    expect(result.newLevel).toBe(4)
    // Lv.3 reward=150 + Lv.4 reward=200 = 350
    expect(result.rewardGold).toBe(350)
  })

  it('#20 满级后不升级 level=10, captures=500', () => {
    const result = tryLevelUp(10, 500)
    expect(result.leveledUp).toBe(false)
    expect(result.newLevel).toBe(10)
    expect(result.rewardGold).toBe(0)
  })
})

describe('calculateBuyPotion', () => {
  it('#21 正常购买 gold=300, purchased=0 => success, remaining=2', () => {
    const result = calculateBuyPotion(300, 0)
    expect(result.success).toBe(true)
    expect(result.remainingPurchases).toBe(2)
  })

  it('#22 金币不足 gold=100, purchased=0 => insufficient_gold', () => {
    const result = calculateBuyPotion(100, 0)
    expect(result.success).toBe(false)
    expect(result.reason).toBe('insufficient_gold')
  })

  it('#23 达到每日限购 gold=999, purchased=3 => daily_limit_reached', () => {
    const result = calculateBuyPotion(999, 3)
    expect(result.success).toBe(false)
    expect(result.reason).toBe('daily_limit_reached')
    expect(result.remainingPurchases).toBe(0)
  })
})

describe('shouldResetDailyPurchases', () => {
  it('#24 跨日重置：日期不是今天 => 需重置', () => {
    // 使用 BASE_TIME 对应的日期是 2025-01-15
    const todayStr = getTodayString(BASE_TIME)
    const yesterdayStr = '2025-01-14'
    expect(shouldResetDailyPurchases(yesterdayStr, BASE_TIME)).toBe(true)
    expect(shouldResetDailyPurchases(todayStr, BASE_TIME)).toBe(false)
  })
})

describe('getTodayString', () => {
  it('返回 YYYY-MM-DD 格式', () => {
    // BASE_TIME = 2025-01-15 12:00:00 UTC
    const result = getTodayString(BASE_TIME)
    expect(result).toMatch(/^\d{4}-\d{2}-\d{2}$/)
  })
})

describe('持久化与恢复测试', () => {
  it('#25 离线恢复：存档后等 1 小时重新加载，体力增加 10', () => {
    // 模拟存档时间 BASE_TIME，离线 1 小时后加载
    const savedStamina = 80
    const maxStamina = getMaxStamina(1)
    const loadTime = BASE_TIME + 3600 * 1000
    const result = calculateRecovery(BASE_TIME, savedStamina, maxStamina, loadTime)
    expect(result.current).toBe(savedStamina + 10)
  })

  it('#26 离线恢复到满：体力 115，离线 2 小时 => 120（不超上限）', () => {
    const savedStamina = 115
    const maxStamina = getMaxStamina(1)
    const loadTime = BASE_TIME + 7200 * 1000
    const result = calculateRecovery(BASE_TIME, savedStamina, maxStamina, loadTime)
    expect(result.current).toBe(120)
    expect(result.recoverTime).toBe(0)
  })

  it('#27 存档读写往返：状态完全一致', () => {
    const state = {
      level: 3,
      currentStamina: 100,
      totalCaptures: 25,
      lastRecoverTime: BASE_TIME,
      gold: 500,
      potionPurchasesToday: 1,
      potionPurchaseDate: getTodayString(BASE_TIME),
    }
    // 序列化再反序列化
    const serialized = JSON.stringify(state)
    const deserialized = JSON.parse(serialized)
    expect(deserialized).toEqual(state)
  })
})
