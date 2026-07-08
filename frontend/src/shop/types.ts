import type { ItemId } from './constants'

/** 道具背包：ItemId → 持有数量 */
export type Inventory = Partial<Record<ItemId, number>>

/** 每日限购记录：ItemId → 今日已购买次数 */
export type DailyPurchaseMap = Partial<Record<ItemId, number>>

/** 签到状态（扩展） */
export interface CheckInState {
  /** 当前连续签到天数（0 = 未签到过） */
  streak: number
  /** 上次签到日期（'YYYY-MM-DD'） */
  lastCheckInDate: string
  /** 累计签到总次数（跨周期累计，成就统计用） */
  totalCheckIns: number
  /** 历史最高连续签到天数（成就统计用） */
  maxStreak: number
  /** 上次断签日期（'YYYY-MM-DD'，空字符串表示从未断签） */
  lastBreakDate: string
}

/** 商店系统状态 */
export interface ShopState {
  /** 道具背包 */
  inventory: Inventory
  /** 签到状态 */
  checkIn: CheckInState
  /** 每日限购记录 */
  dailyPurchases: DailyPurchaseMap
  /** 每日限购日期标记（'YYYY-MM-DD'） */
  dailyPurchaseDate: string
}

/** 购买道具结果 */
export interface BuyResult {
  success: boolean
  /** 失败原因 */
  reason?: 'insufficient_gold' | 'daily_limit_reached'
  /** 购买后剩余金币 */
  remainingGold: number
  /** 购买后该道具剩余每日限购次数（不限购时为 null） */
  remainingDailyPurchases: number | null
}

/** 单日签到奖励定义 */
export interface CheckInReward {
  /** 签到天数（1~7） */
  day: number
  /** 金币奖励 */
  gold: number
  /** 经验值奖励 */
  exp: number
  /** 额外道具（仅第 7 天有值） */
  bonusItem?: ItemId
  /** 是否为满签日（第 7 天） */
  isMilestone: boolean
}

/** 签到面板状态快照（供 UI 渲染用） */
export interface CheckInStatus {
  /** 今日是否已签到 */
  hasCheckedInToday: boolean
  /** 当前连续签到天数 */
  currentStreak: number
  /** 今日签到后将达到的天数（预览用） */
  nextStreak: number
  /** 本周期内已完成签到的天数列表（[1, 2, 3] 表示已签 3 天） */
  completedDays: number[]
  /** 今日是本周期第几天（1~7） */
  todayCycleDay: number
  /** 是否处于断签状态（上次签到不是昨天且今日未签到） */
  isStreakBroken: boolean
  /** 今日签到可获得的奖励 */
  todayReward: CheckInReward
  /** 明日签到可获得的奖励（预览用） */
  tomorrowReward: CheckInReward
  /** 历史最高连签 */
  maxStreak: number
  /** 累计签到次数 */
  totalCheckIns: number
}

/** 签到结果（增强） */
export interface CheckInResult {
  success: boolean
  /** 签到后天数（本次签到后的连续天数） */
  newStreak: number
  /** 获得金币 */
  reward: number
  /** 获得经验值 */
  rewardExp: number
  /** 获得道具（仅第 7 天） */
  rewardItem?: ItemId
  /** 是否可签到 */
  canCheckIn: boolean
  /** 是否发生了断签重置（本次签到因断签而重置为第 1 天） */
  wasReset: boolean
  /** 失败原因 */
  reason?: 'already_checked_in'
}

/** 使用道具结果 */
export interface UseItemResult {
  success: boolean
  reason?: 'not_in_inventory'
}

/** Reducer Action 类型 */
export type ShopAction =
  | { type: 'BUY_ITEM'; itemId: ItemId }
  | { type: 'USE_ITEM'; itemId: ItemId }
  | { type: 'ADD_ITEM'; itemId: ItemId; count: number }
  | { type: 'CHECK_IN'; reward: number; rewardExp: number; newStreak: number; wasReset: boolean; rewardItem?: ItemId }
  | { type: 'RESET_DAILY_PURCHASES'; date: string }
  | { type: 'LOAD_STATE'; state: ShopState }

/** ShopContext 暴露给组件的接口 */
export interface ShopContextValue {
  state: ShopState
  /** 购买道具（扣金币 → 加背包） */
  buyItem: (itemId: ItemId) => BuyResult
  /** 使用道具（从背包消耗 1 个） */
  useItem: (itemId: ItemId) => UseItemResult
  /** 每日签到（增强版） */
  checkIn: () => CheckInResult
  /** 获取签到面板状态快照（供 UI 渲染用） */
  getCheckInStatus: () => CheckInStatus
  /** 查询道具持有数量 */
  getItemCount: (itemId: ItemId) => number
  /** 查询某道具今日已购买次数 */
  getDailyPurchaseCount: (itemId: ItemId) => number
  /** 获取当前捕获增益百分比（0 / 15 / 25） */
  getCaptureBoost: () => number
  /** 消耗捕获增益（投掷后调用） */
  consumeCaptureBoost: () => void
  /** 玩具球是否已激活 */
  isCaptureBoostActive: () => boolean
  /** 添加道具到背包（战斗掉落等） */
  addItem: (itemId: ItemId) => void
}
