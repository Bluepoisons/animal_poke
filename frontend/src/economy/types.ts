import type { RarityTier } from '../types'

// ===== 产出/消耗分类 =====

/** 金币产出来源分类 */
export type GoldSource =
  | 'capture'       // 捕获掉落
  | 'battle_win'    // 战斗胜利
  | 'battle_draw'   // 战斗平局
  | 'battle_lose'   // 战斗失败安慰金
  | 'checkin'       // 每日签到
  | 'levelup'       // 升级奖励
  | 'dispatch'      // 派遣任务
  | 'region_rank'   // 区域排行奖励（预留）
  | 'achievement'   // 成就解锁（预留）
  | 'other'         // 其他

/** 金币消耗去向分类 */
export type GoldSink =
  | 'shop_buy'        // 商店购买道具
  | 'stamina_potion'  // 购买体力药剂
  | 'dispatch_speedup'// 派遣加速
  | 'battle_extra'    // 购买额外战斗场次（预留）
  | 'other'           // 其他

// ===== 经济追踪 =====

/** 单条经济流水记录 */
export interface EconomyLogEntry {
  /** 记录 ID（自增） */
  id: number
  /** 流水类型 */
  type: 'earn' | 'spend'
  /** 金额（正数） */
  amount: number
  /** 来源/去向 */
  category: GoldSource | GoldSink
  /** 时间戳（Unix ms） */
  timestamp: number
  /** 备注（可选，如 "捕获 #000059"） */
  note?: string
}

/** 经济系统完整状态 */
export interface EconomyState {
  /** 累计金币产出 */
  totalEarned: number
  /** 累计金币消耗 */
  totalSpent: number
  /** 流水记录（最多保留 200 条，FIFO 淘汰） */
  logs: EconomyLogEntry[]
  /** 下一条流水 ID */
  nextLogId: number
  /** 今日产出（自然日重置） */
  todayEarned: number
  /** 今日消耗（自然日重置） */
  todaySpent: number
  /** 今日日期标记（'YYYY-MM-DD'） */
  todayDate: string
}

/** 经济统计快照（供 UI 看板使用） */
export interface EconomyStats {
  /** 当前金币余额 */
  currentGold: number
  /** 累计产出 */
  totalEarned: number
  /** 累计消耗 */
  totalSpent: number
  /** 净流入（累计） */
  netFlow: number
  /** 今日产出 */
  todayEarned: number
  /** 今日消耗 */
  todaySpent: number
  /** 今日净流入 */
  todayNetFlow: number
  /** 产出来源分布（累计） */
  earnedBySource: Record<GoldSource, number>
  /** 消耗去向分布（累计） */
  spentBySink: Record<GoldSink, number>
}

/** 经济平衡检查结果 */
export interface BalanceCheckResult {
  /** 产出消耗比（totalEarned / totalSpent，spent=0 时为 Infinity） */
  ratio: number
  /** 是否健康（ratio 在 1.2~3.0 之间） */
  isHealthy: boolean
  /** 状态标签 */
  status: 'deflation' | 'healthy' | 'inflation'
  /** 建议文案 */
  suggestion: string
}

// ===== 派遣任务 =====

/** 派遣任务类型 */
export type DispatchMissionType = 'quick' | 'standard' | 'deep'

/** 派遣任务状态 */
export type DispatchMissionStatus = 'active' | 'completed' | 'collected'

/** 单次派遣任务实例 */
export interface DispatchMission {
  /** 任务实例 ID */
  id: string
  /** 任务类型 */
  type: DispatchMissionType
  /** 派遣的宠物 ID（CardEntry.id） */
  petId: string
  /** 宠物稀有度（影响奖励） */
  petRarity: RarityTier
  /** 绑定城市（可选，影响区域声望） */
  city: string
  /** 开始时间戳（Unix ms） */
  startTime: number
  /** 完成时间戳（Unix ms） */
  endTime: number
  /** 任务状态 */
  status: DispatchMissionStatus
  /** 预期奖励（开始时计算好） */
  rewards: DispatchReward
}

/** 派遣奖励 */
export interface DispatchReward {
  /** 金币 */
  gold: number
  /** 掉落道具 ID（概率掉落） */
  droppedItem?: string
  /** 亲密度增量 */
  affinity: number
}

/** 派遣任务定义（静态） */
export interface DispatchMissionDef {
  type: DispatchMissionType
  name: string
  description: string
  icon: string
  /** 时长（分钟） */
  durationMin: number
  /** 体力消耗 */
  staminaCost: number
  /** 基准金币（Common 宠物基准） */
  baseGold: number
  /** 道具掉落概率 */
  itemDropRate: number
  /** 道具掉落池 */
  itemDropPool: { itemId: string; weight: number }[]
}

// ===== 派遣系统状态 =====

/** 派遣系统完整状态 */
export interface DispatchState {
  /** 当前活跃的派遣任务列表 */
  missions: DispatchMission[]
  /** 今日已派遣次数（自然日重置） */
  todayDispatchCount: number
  /** 今日日期标记 */
  todayDate: string
}

/** 派遣操作结果 */
export interface DispatchResult {
  success: boolean
  reason?: 'insufficient_stamina' | 'no_slots' | 'pet_busy' | 'pet_not_found'
  mission?: DispatchMission
}

/** 派遣领取结果 */
export interface CollectResult {
  success: boolean
  reason?: 'not_completed' | 'not_found'
  rewards?: DispatchReward
}

// ===== Reducer Action 类型 =====

/** EconomyAction */
export type EconomyAction =
  | { type: 'TRACK_EARN'; amount: number; source: GoldSource; note?: string; now: number }
  | { type: 'TRACK_SPEND'; amount: number; sink: GoldSink; note?: string; now: number }
  | { type: 'RESET_DAILY'; date: string }
  | { type: 'LOAD_STATE'; state: EconomyState }

/** DispatchAction */
export type DispatchAction =
  | { type: 'START_MISSION'; mission: DispatchMission; now: number }
  | { type: 'COMPLETE_MISSION'; missionId: string; now: number }
  | { type: 'COLLECT_MISSION'; missionId: string }
  | { type: 'SPEED_UP_MISSION'; missionId: string; now: number }
  | { type: 'RESET_DAILY'; date: string }
  | { type: 'LOAD_STATE'; state: DispatchState }

// ===== Context 暴露接口 =====

/** EconomyContextValue */
export interface EconomyContextValue {
  state: EconomyState
  /** 获取经济统计快照 */
  getStats: (currentGold: number) => EconomyStats
  /** 获取平衡检查结果 */
  getBalanceCheck: () => BalanceCheckResult
  /** 记录金币产出 */
  trackEarn: (amount: number, source: GoldSource, note?: string) => void
  /** 记录金币消耗 */
  trackSpend: (amount: number, sink: GoldSink, note?: string) => void
  /** 获取最近 N 条流水 */
  getRecentLogs: (count: number) => EconomyLogEntry[]
}

/** DispatchContextValue */
export interface DispatchContextValue {
  state: DispatchState
  /** 当前可用派遣槽位数 */
  availableSlots: number
  /** 派遣任务定义列表 */
  missionDefs: DispatchMissionDef[]
  /** 发起派遣 */
  startMission: (missionType: DispatchMissionType, petId: string, petRarity: RarityTier) => DispatchResult
  /** 领取派遣奖励 */
  collectMission: (missionId: string) => CollectResult
  /** 加速派遣（花费金币立即完成） */
  speedUpMission: (missionId: string) => { success: boolean; reason?: 'insufficient_gold' | 'not_found' | 'already_completed' }
  /** 检查并结算已完成的任务 */
  checkCompleted: () => void
  /** 获取宠物的当前派遣任务（判断是否空闲） */
  getPetMission: (petId: string) => DispatchMission | null
  /** 获取倒计时（秒） */
  getMissionCountdown: (missionId: string) => number
}
