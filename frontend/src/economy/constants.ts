import type { DispatchMissionType, GoldSource, GoldSink, DispatchMissionDef } from './types'
import type { RarityTier } from '../types'

// ===== 经济追踪 =====

/** localStorage 存储 key */
export const ECONOMY_STORAGE_KEY = 'animal_poke_economy'

/** 流水记录最大保留条数 */
export const MAX_LOG_ENTRIES = 200

/** 经济健康区间 */
export const HEALTHY_RATIO_MIN = 1.2
export const HEALTHY_RATIO_MAX = 3.0

// ===== 派遣系统 =====

/** localStorage 存储 key */
export const DISPATCH_STORAGE_KEY = 'animal_poke_dispatch'

/** 派遣定时检查间隔（毫秒），每分钟检查一次 */
export const DISPATCH_CHECK_INTERVAL_MS = 60_000

/** 派遣加速费用（金币） */
export const DISPATCH_SPEEDUP_COST = 50

/** 每日派遣次数上限 */
export const DAILY_DISPATCH_LIMIT = 6

/** 稀有度金币倍率（与 BATTLE_GOLD_REWARDS 对齐） */
export const DISPATCH_RARITY_GOLD_MULTIPLIER: Record<RarityTier, number> = {
  common: 1.0,
  uncommon: 1.67,   // 15 × 1.67 ≈ 25
  rare: 2.67,       // 15 × 2.67 ≈ 40
  epic: 4.67,       // 15 × 4.67 ≈ 70
  legendary: 8.0,   // 15 × 8.0 = 120
}

/** 派遣亲密度奖励 */
export const DISPATCH_AFFINITY_REWARD = 2

/** 派遣道具掉落概率 */
export const DISPATCH_ITEM_DROP_RATE = 0.20

/** 派遣道具掉落池 */
export const DISPATCH_ITEM_DROP_POOL: { itemId: string; weight: number }[] = [
  { itemId: 'toy_ball', weight: 35 },
  { itemId: 'food_pack', weight: 30 },
  { itemId: 'bait', weight: 20 },
  { itemId: 'cold_medicine', weight: 10 },
  { itemId: 'premium_toy_ball', weight: 5 },
]

// ===== 派遣任务定义 =====

export const DISPATCH_MISSION_DEFS: DispatchMissionDef[] = [
  {
    type: 'quick',
    name: '快速探索',
    description: '派宠物在附近快速搜寻，30 分钟返回',
    icon: '⚡',
    durationMin: 30,
    staminaCost: 20,
    baseGold: 15,
    itemDropRate: 0.15,
    itemDropPool: DISPATCH_ITEM_DROP_POOL,
  },
  {
    type: 'standard',
    name: '标准探索',
    description: '派宠物去周边探索，1 小时返回',
    icon: '🧭',
    durationMin: 60,
    staminaCost: 20,
    baseGold: 25,
    itemDropRate: 0.20,
    itemDropPool: DISPATCH_ITEM_DROP_POOL,
  },
  {
    type: 'deep',
    name: '深度探索',
    description: '派宠物深入未知区域，2 小时返回，高收益',
    icon: '🗺️',
    durationMin: 120,
    staminaCost: 20,
    baseGold: 40,
    itemDropRate: 0.25,
    itemDropPool: DISPATCH_ITEM_DROP_POOL,
  },
]

/** 派遣任务定义映射（type → def） */
export const DISPATCH_MISSION_MAP: Record<DispatchMissionType, DispatchMissionDef> =
  Object.fromEntries(DISPATCH_MISSION_DEFS.map(d => [d.type, d])) as Record<DispatchMissionType, DispatchMissionDef>

// ===== 派遣槽位计算 =====

/**
 * 根据等级计算派遣槽位上限
 * Lv1-2: 1, Lv3-5: 2, Lv6-8: 3, Lv9-10: 4
 */
export function getMaxDispatchSlots(level: number): number {
  return Math.min(1 + Math.floor(level / 3), 4)
}

// ===== 产出/消耗初始分布 =====

export const INITIAL_EARNED_BY_SOURCE: Record<GoldSource, number> = {
  capture: 0, battle_win: 0, battle_draw: 0, battle_lose: 0,
  checkin: 0, levelup: 0, dispatch: 0, region_rank: 0, achievement: 0, other: 0,
}

export const INITIAL_SPENT_BY_SINK: Record<GoldSink, number> = {
  shop_buy: 0, stamina_potion: 0, dispatch_speedup: 0, battle_extra: 0, other: 0,
}

// ===== 一天的毫秒数 =====

export const DAY_MS = 24 * 60 * 60 * 1000
