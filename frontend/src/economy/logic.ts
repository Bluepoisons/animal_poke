import {
  DISPATCH_RARITY_GOLD_MULTIPLIER,
  DISPATCH_AFFINITY_REWARD,
  DISPATCH_MISSION_MAP,
  HEALTHY_RATIO_MIN,
  HEALTHY_RATIO_MAX,
  INITIAL_EARNED_BY_SOURCE,
  INITIAL_SPENT_BY_SINK,
  getMaxDispatchSlots,
} from './constants'
import type {
  DispatchMissionType,
  DispatchReward,
  DispatchMission,
  DispatchState,
  EconomyState,
  EconomyStats,
  BalanceCheckResult,
  GoldSource,
  GoldSink,
} from './types'
import type { RarityTier } from '../types'

// ===== 日期工具 =====

/**
 * 获取今日日期标记（自然日，格式 'YYYY-MM-DD'）
 */
export function getTodayString(now?: number): string {
  const date = now ? new Date(now) : new Date()
  const y = date.getFullYear()
  const m = String(date.getMonth() + 1).padStart(2, '0')
  const d = String(date.getDate()).padStart(2, '0')
  return `${y}-${m}-${d}`
}

/** 检查是否需要重置每日统计 */
export function shouldResetDaily(date: string, now?: number): boolean {
  return date !== getTodayString(now)
}

// ===== 派遣奖励计算 =====

/**
 * 计算派遣任务奖励（纯函数）
 *
 * 金币 = 任务基准金币 × 稀有度倍率
 * 道具 = 按概率掉落（使用传入的 rng 确保可测试）
 * 亲密度 = 固定 2
 */
export function calculateDispatchReward(
  missionType: DispatchMissionType,
  rarity: RarityTier,
  rng: () => number = Math.random
): DispatchReward {
  const def = DISPATCH_MISSION_MAP[missionType]
  const gold = Math.round(def.baseGold * DISPATCH_RARITY_GOLD_MULTIPLIER[rarity])

  // 道具掉落判定
  let droppedItem: string | undefined
  if (rng() < def.itemDropRate) {
    droppedItem = rollDispatchItemDrop(def.itemDropPool, rng)
  }

  return {
    gold,
    droppedItem,
    affinity: DISPATCH_AFFINITY_REWARD,
  }
}

/**
 * 加权随机选取掉落道具
 */
export function rollDispatchItemDrop(
  pool: { itemId: string; weight: number }[],
  rng: () => number = Math.random
): string {
  const total = pool.reduce((sum, item) => sum + item.weight, 0)
  let rand = rng() * total
  for (const item of pool) {
    rand -= item.weight
    if (rand <= 0) return item.itemId
  }
  return pool[pool.length - 1].itemId
}

// ===== 创建派遣任务实例 =====

/**
 * 创建派遣任务实例
 */
export function createMission(
  missionType: DispatchMissionType,
  petId: string,
  petRarity: RarityTier,
  city: string,
  now: number,
  rng: () => number = Math.random
): DispatchMission {
  const def = DISPATCH_MISSION_MAP[missionType]
  const durationMs = def.durationMin * 60 * 1000
  const rewards = calculateDispatchReward(missionType, petRarity, rng)

  return {
    id: `dispatch_${now}_${Math.floor(rng() * 10000)}`,
    type: missionType,
    petId,
    petRarity,
    city,
    startTime: now,
    endTime: now + durationMs,
    status: 'active',
    rewards,
  }
}

// ===== 任务完成检查 =====

/**
 * 检查派遣任务是否已完成（时间到期）
 */
export function isMissionCompleted(mission: DispatchMission, now: number = Date.now()): boolean {
  return mission.status === 'active' && now >= mission.endTime
}

// ===== 倒计时 =====

/**
 * 获取派遣任务剩余倒计时（秒）
 * 已完成返回 0
 */
export function getMissionCountdown(mission: DispatchMission, now: number = Date.now()): number {
  if (now >= mission.endTime) return 0
  return Math.ceil((mission.endTime - now) / 1000)
}

// ===== 槽位计算 =====

/**
 * 计算当前可用派遣槽位数
 * 可用 = 上限 - 活跃任务数（含已完成未领取）
 */
export function getAvailableSlots(state: DispatchState, level: number): number {
  const maxSlots = getMaxDispatchSlots(level)
  const activeCount = state.missions.filter(m => m.status !== 'collected').length
  return Math.max(0, maxSlots - activeCount)
}

// ===== 宠物派遣状态 =====

/**
 * 获取宠物的当前派遣任务
 * @returns 派遣任务或 null（空闲）
 */
export function getPetMission(state: DispatchState, petId: string): DispatchMission | null {
  return state.missions.find(m => m.petId === petId && m.status !== 'collected') ?? null
}

// ===== 经济平衡检查 =====

/**
 * 经济平衡检查
 *
 * ratio = totalEarned / totalSpent
 * - ratio < 1.2 → deflation（通缩）
 * - 1.2 ≤ ratio ≤ 3.0 → healthy（健康）
 * - ratio > 3.0 → inflation（通胀）
 */
export function balanceCheck(state: EconomyState): BalanceCheckResult {
  const ratio = state.totalSpent > 0
    ? state.totalEarned / state.totalSpent
    : Infinity

  let status: BalanceCheckResult['status']
  let isHealthy: boolean
  let suggestion: string

  if (ratio === Infinity) {
    status = 'inflation'
    isHealthy = false
    suggestion = '尚无消耗记录，建议增加道具购买或派遣加速'
  } else if (ratio < HEALTHY_RATIO_MIN) {
    status = 'deflation'
    isHealthy = false
    suggestion = `产出消耗比 ${ratio.toFixed(2)} 偏低，消耗过多或产出不足，建议增加派遣或提高捕获频率`
  } else if (ratio > HEALTHY_RATIO_MAX) {
    status = 'inflation'
    isHealthy = false
    suggestion = `产出消耗比 ${ratio.toFixed(2)} 偏高，产出过剩，建议增加商店购买或体力药剂消耗`
  } else {
    status = 'healthy'
    isHealthy = true
    suggestion = `经济健康，产出消耗比 ${ratio.toFixed(2)}`
  }

  return { ratio, isHealthy, status, suggestion }
}

// ===== 统计产出来源分布 =====

/**
 * 从流水日志中统计产出来源分布
 */
export function calculateEarnedBySource(state: EconomyState): Record<GoldSource, number> {
  const result = { ...INITIAL_EARNED_BY_SOURCE }
  for (const log of state.logs) {
    if (log.type === 'earn') {
      result[log.category as GoldSource] = (result[log.category as GoldSource] ?? 0) + log.amount
    }
  }
  return result
}

/**
 * 从流水日志中统计消耗去向分布
 */
export function calculateSpentBySink(state: EconomyState): Record<GoldSink, number> {
  const result = { ...INITIAL_SPENT_BY_SINK }
  for (const log of state.logs) {
    if (log.type === 'spend') {
      result[log.category as GoldSink] = (result[log.category as GoldSink] ?? 0) + log.amount
    }
  }
  return result
}

// ===== 经济统计快照 =====

/**
 * 生成经济统计快照
 */
export function getEconomyStats(state: EconomyState, currentGold: number): EconomyStats {
  return {
    currentGold,
    totalEarned: state.totalEarned,
    totalSpent: state.totalSpent,
    netFlow: state.totalEarned - state.totalSpent,
    todayEarned: state.todayEarned,
    todaySpent: state.todaySpent,
    todayNetFlow: state.todayEarned - state.todaySpent,
    earnedBySource: calculateEarnedBySource(state),
    spentBySink: calculateSpentBySink(state),
  }
}

// ===== 加速费用计算 =====

/**
 * 计算加速费用
 * 当前为固定费用 50 金币（后续可按剩余时间递减）
 */
export function getSpeedUpCost(_mission: DispatchMission): number {
  return 50
}
