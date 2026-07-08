import { describe, it, expect } from 'vitest'
import {
  getTodayString,
  shouldResetDaily,
  calculateDispatchReward,
  rollDispatchItemDrop,
  createMission,
  isMissionCompleted,
  getMissionCountdown,
  getAvailableSlots,
  getPetMission,
  balanceCheck,
  getEconomyStats,
  calculateEarnedBySource,
  calculateSpentBySink,
  getSpeedUpCost,
} from './logic'
import { getMaxDispatchSlots, DISPATCH_SPEEDUP_COST } from './constants'
import type { EconomyState, DispatchState } from './types'

const NOW = 1700000000000

// ===== 辅助函数 =====
function makeEconomyState(overrides: Partial<EconomyState> = {}): EconomyState {
  return {
    totalEarned: 0,
    totalSpent: 0,
    logs: [],
    nextLogId: 1,
    todayEarned: 0,
    todaySpent: 0,
    todayDate: getTodayString(NOW),
    ...overrides,
  }
}

function makeDispatchState(overrides: Partial<DispatchState> = {}): DispatchState {
  return {
    missions: [],
    todayDispatchCount: 0,
    todayDate: getTodayString(NOW),
    ...overrides,
  }
}

// ===== 日期工具测试 =====
describe('getTodayString', () => {
  it('返回 YYYY-MM-DD 格式', () => {
    const result = getTodayString(NOW)
    expect(result).toMatch(/^\d{4}-\d{2}-\d{2}$/)
  })
})

describe('shouldResetDaily', () => {
  it('不同日期返回 true', () => {
    expect(shouldResetDaily('2026-07-07', NOW)).toBe(true)
  })

  it('相同日期返回 false', () => {
    const today = getTodayString(NOW)
    expect(shouldResetDaily(today, NOW)).toBe(false)
  })
})

// ===== 派遣奖励计算 =====
describe('calculateDispatchReward', () => {
  it('Common 快速探索：gold=15, affinity=2', () => {
    const reward = calculateDispatchReward('quick', 'common', () => 0.99)
    expect(reward.gold).toBe(15)
    expect(reward.affinity).toBe(2)
  })

  it('Legendary 深度探索：gold=320', () => {
    const reward = calculateDispatchReward('deep', 'legendary', () => 0.99)
    expect(reward.gold).toBe(320) // 40 × 8.0
    expect(reward.affinity).toBe(2)
  })

  it('道具掉落：rng < dropRate 时掉落', () => {
    const reward = calculateDispatchReward('standard', 'common', () => 0.01)
    expect(reward.droppedItem).toBeDefined()
  })

  it('无道具掉落：rng ≥ dropRate 时不掉落', () => {
    const reward = calculateDispatchReward('standard', 'common', () => 0.99)
    expect(reward.droppedItem).toBeUndefined()
  })
})

// ===== 道具掉落池 =====
describe('rollDispatchItemDrop', () => {
  it('返回池中存在的 itemId', () => {
    const pool = [
      { itemId: 'toy_ball', weight: 50 },
      { itemId: 'food_pack', weight: 50 },
    ]
    const result = rollDispatchItemDrop(pool, () => 0.3)
    expect(['toy_ball', 'food_pack']).toContain(result)
  })

  it('rng=0 返回第一个池中道具', () => {
    const pool = [
      { itemId: 'toy_ball', weight: 50 },
      { itemId: 'food_pack', weight: 50 },
    ]
    const result = rollDispatchItemDrop(pool, () => 0)
    expect(result).toBe('toy_ball')
  })

  it('rng 接近 1 返回最后一个池中道具', () => {
    const pool = [
      { itemId: 'toy_ball', weight: 50 },
      { itemId: 'food_pack', weight: 50 },
    ]
    const result = rollDispatchItemDrop(pool, () => 0.999)
    expect(result).toBe('food_pack')
  })
})

// ===== 创建派遣任务 =====
describe('createMission', () => {
  it('正确创建任务字段', () => {
    const mission = createMission('quick', 'pet001', 'common', '宁波市', NOW, () => 0.5)
    expect(mission.type).toBe('quick')
    expect(mission.petId).toBe('pet001')
    expect(mission.petRarity).toBe('common')
    expect(mission.city).toBe('宁波市')
    expect(mission.startTime).toBe(NOW)
    expect(mission.status).toBe('active')
  })

  it('快速探索结束时间 = startTime + 30min', () => {
    const mission = createMission('quick', 'pet001', 'common', '宁波市', NOW, () => 0.5)
    expect(mission.endTime).toBe(NOW + 30 * 60 * 1000)
  })

  it('深度探索结束时间 = startTime + 120min', () => {
    const mission = createMission('deep', 'pet001', 'common', '宁波市', NOW, () => 0.5)
    expect(mission.endTime).toBe(NOW + 120 * 60 * 1000)
  })
})

// ===== 任务完成检查 =====
describe('isMissionCompleted', () => {
  it('未到期返回 false', () => {
    const mission = createMission('quick', 'pet001', 'common', '', NOW, () => 0.5)
    expect(isMissionCompleted(mission, NOW + 1000)).toBe(false)
  })

  it('到期返回 true', () => {
    const mission = createMission('quick', 'pet001', 'common', '', NOW, () => 0.5)
    expect(isMissionCompleted(mission, NOW + 31 * 60 * 1000)).toBe(true)
  })
})

// ===== 倒计时 =====
describe('getMissionCountdown', () => {
  it('返回剩余秒数', () => {
    const mission = createMission('quick', 'pet001', 'common', '', NOW, () => 0.5)
    // 30min 任务，过了 10min，剩 20min = 1200 秒
    expect(getMissionCountdown(mission, NOW + 10 * 60 * 1000)).toBe(1200)
  })

  it('已完成返回 0', () => {
    const mission = createMission('quick', 'pet001', 'common', '', NOW, () => 0.5)
    expect(getMissionCountdown(mission, NOW + 31 * 60 * 1000)).toBe(0)
  })
})

// ===== 槽位计算 =====
describe('getAvailableSlots', () => {
  it('Lv5 无任务返回 2', () => {
    const state = makeDispatchState()
    expect(getAvailableSlots(state, 5)).toBe(2)
  })

  it('Lv5 有 1 个活跃任务返回 1', () => {
    const mission = createMission('quick', 'pet001', 'common', '', NOW, () => 0.5)
    const state = makeDispatchState({ missions: [mission] })
    expect(getAvailableSlots(state, 5)).toBe(1)
  })

  it('Lv1 上限为 1', () => {
    const state = makeDispatchState()
    expect(getAvailableSlots(state, 1)).toBe(1)
  })
})

// ===== 宠物派遣状态 =====
describe('getPetMission', () => {
  it('空闲宠物返回 null', () => {
    const state = makeDispatchState()
    expect(getPetMission(state, 'pet001')).toBeNull()
  })

  it('派遣中宠物返回任务', () => {
    const mission = createMission('quick', 'pet001', 'common', '', NOW, () => 0.5)
    const state = makeDispatchState({ missions: [mission] })
    expect(getPetMission(state, 'pet001')).not.toBeNull()
  })

  it('已领取奖励的宠物返回 null', () => {
    const mission = createMission('quick', 'pet001', 'common', '', NOW, () => 0.5)
    const collectedMission = { ...mission, status: 'collected' as const }
    const state = makeDispatchState({ missions: [collectedMission] })
    expect(getPetMission(state, 'pet001')).toBeNull()
  })
})

// ===== 经济平衡检查 =====
describe('balanceCheck', () => {
  it('ratio=2.0 为健康', () => {
    const state = makeEconomyState({ totalEarned: 200, totalSpent: 100 })
    const result = balanceCheck(state)
    expect(result.ratio).toBe(2.0)
    expect(result.isHealthy).toBe(true)
    expect(result.status).toBe('healthy')
  })

  it('ratio=5.0 为通胀', () => {
    const state = makeEconomyState({ totalEarned: 500, totalSpent: 100 })
    const result = balanceCheck(state)
    expect(result.isHealthy).toBe(false)
    expect(result.status).toBe('inflation')
  })

  it('ratio=0.5 为通缩', () => {
    const state = makeEconomyState({ totalEarned: 50, totalSpent: 100 })
    const result = balanceCheck(state)
    expect(result.isHealthy).toBe(false)
    expect(result.status).toBe('deflation')
  })

  it('无消耗记录为通胀（Infinity）', () => {
    const state = makeEconomyState({ totalEarned: 100, totalSpent: 0 })
    const result = balanceCheck(state)
    expect(result.ratio).toBe(Infinity)
    expect(result.status).toBe('inflation')
  })
})

// ===== 槽位上限 =====
describe('getMaxDispatchSlots', () => {
  it('Lv1 → 1', () => {
    expect(getMaxDispatchSlots(1)).toBe(1)
  })
  it('Lv3 → 2', () => {
    expect(getMaxDispatchSlots(3)).toBe(2)
  })
  it('Lv6 → 3', () => {
    expect(getMaxDispatchSlots(6)).toBe(3)
  })
  it('Lv9 → 4', () => {
    expect(getMaxDispatchSlots(9)).toBe(4)
  })
  it('Lv10 → 4（上限）', () => {
    expect(getMaxDispatchSlots(10)).toBe(4)
  })
})

// ===== 统计快照 =====
describe('getEconomyStats', () => {
  it('正确计算净流入', () => {
    const state = makeEconomyState({ totalEarned: 500, totalSpent: 300, todayEarned: 100, todaySpent: 50 })
    const stats = getEconomyStats(state, 200)
    expect(stats.currentGold).toBe(200)
    expect(stats.netFlow).toBe(200)
    expect(stats.todayNetFlow).toBe(50)
  })
})

// ===== 产出来源分布 =====
describe('calculateEarnedBySource', () => {
  it('从流水日志正确统计来源分布', () => {
    const state = makeEconomyState({
      totalEarned: 80,
      logs: [
        { id: 1, type: 'earn', amount: 30, category: 'capture', timestamp: NOW },
        { id: 2, type: 'earn', amount: 50, category: 'dispatch', timestamp: NOW },
      ],
    })
    const result = calculateEarnedBySource(state)
    expect(result.capture).toBe(30)
    expect(result.dispatch).toBe(50)
    expect(result.battle_win).toBe(0)
  })
})

// ===== 消耗去向分布 =====
describe('calculateSpentBySink', () => {
  it('从流水日志正确统计去向分布', () => {
    const state = makeEconomyState({
      totalSpent: 200,
      logs: [
        { id: 1, type: 'spend', amount: 50, category: 'shop_buy', timestamp: NOW },
        { id: 2, type: 'spend', amount: 150, category: 'stamina_potion', timestamp: NOW },
      ],
    })
    const result = calculateSpentBySink(state)
    expect(result.shop_buy).toBe(50)
    expect(result.stamina_potion).toBe(150)
    expect(result.dispatch_speedup).toBe(0)
  })
})

// ===== 加速费用 =====
describe('getSpeedUpCost', () => {
  it('返回固定加速费用', () => {
    const mission = createMission('quick', 'pet001', 'common', '', NOW, () => 0.5)
    expect(getSpeedUpCost(mission)).toBe(DISPATCH_SPEEDUP_COST)
  })
})
