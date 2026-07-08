import type { RarityTier, SpeciesType } from '../types'
import type { WeatherType } from '../weather/types'

/** 成就类别 */
export type AchievementCategory =
  | 'collection'
  | 'battle'
  | 'exploration'
  | 'social'
  | 'milestone'

/** 成就稀有度（影响视觉样式） */
export type AchievementRarity = 'bronze' | 'silver' | 'gold' | 'platinum'

/** 成就奖励 */
export interface AchievementReward {
  /** 金币奖励（0 表示无） */
  gold: number
  /** 称号奖励（可选） */
  title?: string
  /** 道具奖励（可选，ItemId） */
  item?: string
}

/** 成就定义（静态配置） */
export interface AchievementDef {
  /** 成就唯一 ID */
  id: string
  /** 成就名称（中文） */
  name: string
  /** 成就描述 */
  description: string
  /** 成就类别 */
  category: AchievementCategory
  /** 成就稀有度 */
  rarity: AchievementRarity
  /** 图标 emoji */
  icon: string
  /** 目标值 */
  target: number
  /** 奖励 */
  reward: AchievementReward
  /** 解锁条件类型 */
  condition: AchievementCondition
}

/** 成就解锁条件类型 */
export type AchievementCondition =
  | { type: 'total_captures'; target: number }
  | { type: 'captures_by_rarity'; rarity: RarityTier; target: number }
  | { type: 'captures_by_species'; species: SpeciesType; target: number }
  | { type: 'all_species_diversity'; target: number }
  | { type: 'total_battles_won'; target: number }
  | { type: 'total_battles'; target: number }
  | { type: 'win_streak'; target: number }
  | { type: 'level_reached'; target: number }
  | { type: 'check_in_days'; target: number }
  | { type: 'cities_visited'; target: number }
  | { type: 'weather_types_experienced'; target: number }
  | { type: 'rain_captures_no_cold'; target: number }
  | { type: 'legendary_captured' }
  | { type: 'all_weather_types' }

/** 成就解锁记录 */
export interface UnlockedAchievement {
  /** 成就 ID */
  id: string
  /** 解锁时间戳（Unix ms） */
  unlockedAt: number
}

/** 成就系统状态 */
export interface AchievementState {
  /** 已解锁成就列表 */
  unlocked: UnlockedAchievement[]
  /** 已解锁称号列表 */
  unlockedTitles: string[]
  /** 当前佩戴的称号 */
  activeTitle: string | null
  /** 成就弹窗队列（待展示的新解锁成就） */
  pendingNotifications: string[]
}

/** 成就进度信息（UI 展示用） */
export interface AchievementProgress {
  /** 成就 ID */
  id: string
  /** 当前进度值 */
  current: number
  /** 目标值 */
  target: number
  /** 是否已解锁 */
  unlocked: boolean
  /** 进度百分比 0~100 */
  percent: number
}

/** 成就检查输入（从各 Context 汇总的统计数据） */
export interface AchievementStats {
  totalCaptures: number
  totalBattlesWon: number
  totalBattles: number
  currentWinStreak: number
  maxWinStreak: number
  level: number
  checkInStreak: number
  citiesVisited: number
  weatherTypesExperienced: WeatherType[]
  /** 按稀有度统计捕获数 */
  capturesByRarity: Record<RarityTier, number>
  /** 按物种统计捕获数 */
  capturesBySpecies: Record<SpeciesType, number>
  /** 是否捕获过传说级 */
  hasLegendary: boolean
  /** 雨天捕获无感冒次数 */
  rainCapturesNoCold: number
}

/** 成就解锁结果 */
export interface AchievementCheckResult {
  /** 新解锁的成就 ID 列表 */
  newlyUnlocked: string[]
  /** 是否有变化 */
  changed: boolean
}

/** Reducer Action */
export type AchievementAction =
  | { type: 'UNLOCK_ACHIEVEMENTS'; ids: string[]; unlockedAt: number }
  | { type: 'SET_ACTIVE_TITLE'; title: string }
  | { type: 'ENQUEUE_NOTIFICATION'; id: string }
  | { type: 'DEQUEUE_NOTIFICATION' }
  | { type: 'LOAD_STATE'; state: AchievementState }

/** AchievementContextValue */
export interface AchievementContextValue {
  state: AchievementState
  /** 所有成就定义列表 */
  definitions: AchievementDef[]
  /** 检查并解锁成就（传入当前统计数据） */
  checkAchievements: (stats: AchievementStats) => AchievementCheckResult
  /** 获取单个成就进度 */
  getProgress: (id: string, stats: AchievementStats) => AchievementProgress
  /** 获取所有成就进度列表 */
  getAllProgress: (stats: AchievementStats) => AchievementProgress[]
  /** 按类别获取成就进度 */
  getByCategory: (category: AchievementCategory, stats: AchievementStats) => AchievementProgress[]
  /** 获取已解锁成就数量 */
  getUnlockedCount: () => number
  /** 获取总成就数量 */
  getTotalCount: () => number
  /** 设置当前佩戴称号 */
  setActiveTitle: (title: string | null) => void
  /** 消费一个待展示通知 */
  consumeNotification: () => string | null
}
