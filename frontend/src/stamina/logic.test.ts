import { describe, it, expect } from 'vitest'
import {
  getMaxStamina,
  getLevelForCaptures,
  getLevelForExp,
  calculateRecovery,
  tryLevelUp,
  canConsume,
  getTodayString,
  shouldResetDailyPurchases,
  calculateBuyPotion,
  getExpProgress,
  getCaptureXp,
  getBattleXp,
  migrateState,
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

describe('tryLevelUp (exp 驱动)', () => {
  it('#17 未达升级条件 level=2, exp=150 => 不升级', () => {
    const result = tryLevelUp(2, 150)
    expect(result.leveledUp).toBe(false)
    expect(result.newLevel).toBe(2)
    expect(result.rewardGold).toBe(0)
    expect(result.crossedLevels).toEqual([])
  })

  it('#18 刚好升级一级 level=2, exp=250 => Lv.3, rewardGold=150', () => {
    const result = tryLevelUp(2, 250)
    expect(result.leveledUp).toBe(true)
    expect(result.newLevel).toBe(3)
    expect(result.rewardGold).toBe(150)
    expect(result.crossedLevels).toEqual([3])
  })

  it('#19 跨级升级 level=2, exp=500 => Lv.4, rewardGold=350', () => {
    const result = tryLevelUp(2, 500)
    expect(result.leveledUp).toBe(true)
    expect(result.newLevel).toBe(4)
    // Lv.3 reward=150 + Lv.4 reward=200 = 350
    expect(result.rewardGold).toBe(350)
    expect(result.crossedLevels).toEqual([3, 4])
  })

  it('#20 满级后不升级 level=10, exp=9999', () => {
    const result = tryLevelUp(10, 9999)
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
    const todayStr = getTodayString(BASE_TIME)
    const yesterdayStr = '2025-01-14'
    expect(shouldResetDailyPurchases(yesterdayStr, BASE_TIME)).toBe(true)
    expect(shouldResetDailyPurchases(todayStr, BASE_TIME)).toBe(false)
  })
})

describe('getTodayString', () => {
  it('返回 YYYY-MM-DD 格式', () => {
    const result = getTodayString(BASE_TIME)
    expect(result).toMatch(/^\d{4}-\d{2}-\d{2}$/)
  })
})

describe('持久化与恢复测试', () => {
  it('#25 离线恢复：存档后等 1 小时重新加载，体力增加 10', () => {
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
      exp: 250,
      currentStamina: 100,
      totalCaptures: 25,
      lastRecoverTime: BASE_TIME,
      gold: 500,
      potionPurchasesToday: 1,
      potionPurchaseDate: getTodayString(BASE_TIME),
      totalBattlesWon: 5,
      totalBattles: 10,
      currentWinStreak: 2,
      maxWinStreak: 3,
    }
    const serialized = JSON.stringify(state)
    const deserialized = JSON.parse(serialized)
    expect(deserialized).toEqual(state)
  })
})

// ===== XP 系统新增测试 =====

describe('getLevelForExp', () => {
  it('#28 exp=0 返回 Lv.1', () => {
    expect(getLevelForExp(0)).toBe(1)
  })

  it('#29 exp=100 刚好升 Lv.2', () => {
    expect(getLevelForExp(100)).toBe(2)
  })

  it('#30 exp=500 跨级到 Lv.4', () => {
    expect(getLevelForExp(500)).toBe(4)
  })

  it('#31 exp=9999 满级后仍为 Lv.10', () => {
    expect(getLevelForExp(9999)).toBe(10)
  })
})

describe('tryLevelUp (exp 驱动)', () => {
  it('#32 未达升级条件 level=2, exp=150 => 不升级', () => {
    const result = tryLevelUp(2, 150)
    expect(result.leveledUp).toBe(false)
    expect(result.newLevel).toBe(2)
    expect(result.rewardGold).toBe(0)
    expect(result.crossedLevels).toEqual([])
  })

  it('#33 刚好升级一级 level=2, exp=250 => Lv.3, rewardGold=150', () => {
    const result = tryLevelUp(2, 250)
    expect(result.leveledUp).toBe(true)
    expect(result.newLevel).toBe(3)
    expect(result.rewardGold).toBe(150)
    expect(result.crossedLevels).toEqual([3])
  })

  it('#34 跨级升级 level=2, exp=500 => Lv.4, rewardGold=350', () => {
    const result = tryLevelUp(2, 500)
    expect(result.leveledUp).toBe(true)
    expect(result.newLevel).toBe(4)
    expect(result.rewardGold).toBe(350)
    expect(result.crossedLevels).toEqual([3, 4])
  })

  it('#35 满级后不升级 level=10, exp=9999', () => {
    const result = tryLevelUp(10, 9999)
    expect(result.leveledUp).toBe(false)
    expect(result.newLevel).toBe(10)
    expect(result.rewardGold).toBe(0)
  })
})

describe('getExpProgress', () => {
  it('#36 Lv.1 exp=0 => progress=0', () => {
    const result = getExpProgress(1, 0)
    expect(result.currentLevelExp).toBe(0)
    expect(result.nextLevelExp).toBe(100)
    expect(result.progress).toBe(0)
  })

  it('#37 Lv.1 exp=50 => progress=50', () => {
    const result = getExpProgress(1, 50)
    expect(result.currentLevelExp).toBe(50)
    expect(result.nextLevelExp).toBe(100)
    expect(result.progress).toBe(50)
  })

  it('#38 Lv.5 exp=700 => 刚好升级边界 progress=0（新等级起始）', () => {
    const result = getExpProgress(5, 700)
    expect(result.currentLevelExp).toBe(0)
    expect(result.nextLevelExp).toBe(300)
    expect(result.progress).toBe(0)
  })

  it('#39 Lv.10 满级 => progress=100', () => {
    const result = getExpProgress(10, 5000)
    expect(result.progress).toBe(100)
  })
})

describe('getCaptureXp', () => {
  it('#40 common 捕获得 8 XP', () => {
    expect(getCaptureXp('common')).toBe(8)
  })

  it('#41 legendary 捕获得 120 XP', () => {
    expect(getCaptureXp('legendary')).toBe(120)
  })
})

describe('getBattleXp', () => {
  it('#42 胜利 common 敌 => 20 XP', () => {
    expect(getBattleXp('win', 'common')).toBe(20)
  })

  it('#43 胜利 legendary 敌 => 40 XP', () => {
    expect(getBattleXp('win', 'legendary')).toBe(40)
  })

  it('#44 失败 => 5 XP', () => {
    expect(getBattleXp('lose', 'rare')).toBe(5)
  })

  it('#45 平局 => 10 XP', () => {
    expect(getBattleXp('draw', 'epic')).toBe(10)
  })
})

describe('migrateState', () => {
  it('#46 旧存档无 exp 字段，按 totalCaptures×10 推算', () => {
    const oldSave = { level: 3, totalCaptures: 25 } as any
    const migrated = migrateState(oldSave)
    expect(migrated.exp).toBe(250)
    expect(migrated.level).toBe(3)
  })

  it('#47 新存档有 exp 字段，保持不变', () => {
    const newSave = { level: 2, exp: 150, totalCaptures: 12 } as any
    const migrated = migrateState(newSave)
    expect(migrated.exp).toBe(150)
    expect(migrated.level).toBe(2)
  })

  it('#48 缺失字段补全默认值', () => {
    const partial = { level: 1 } as any
    const migrated = migrateState(partial)
    expect(migrated.currentStamina).toBe(120)
    expect(migrated.gold).toBe(0)
    expect(migrated.exp).toBe(0)
  })
})
