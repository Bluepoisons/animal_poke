import type { StatusType } from './types'

/** 一天的毫秒数 */
export const DAY_MS = 24 * 60 * 60 * 1000

// ===== 感冒参数 =====

/** 感冒持续天数 */
export const COLD_DURATION_DAYS = 5

/** 感冒属性修正倍率（全属性 -35%） */
export const COLD_STAT_MULTIPLIER = 0.65

// ===== 愉悦参数 =====

/** 愉悦属性修正倍率（全属性 +5%）— 仅供 UI 展示参考，实际修正由天气系统处理 */
export const PLEASURE_STAT_MULTIPLIER = 1.05

// ===== 永久损伤参数（已下线，代码保留） =====

/** 永久损伤功能开关（里程碑 7.3 标注「已下线」） */
export const PERMANENT_DAMAGE_ENABLED = false

/** 永久损伤最低概率（2%） */
export const PERMANENT_DAMAGE_MIN_RATE = 0.02

/** 永久损伤最高概率（5%） */
export const PERMANENT_DAMAGE_MAX_RATE = 0.05

/** 永久损伤属性修正倍率（全属性 -5%） */
export const PERMANENT_DAMAGE_MULTIPLIER = 0.95

// ===== 恢复参数 =====

/** 自然恢复完全恢复的最低概率（95%） */
export const FULL_RECOVERY_MIN_RATE = 0.95

/** 自然恢复完全恢复的最高概率（98%） */
export const FULL_RECOVERY_MAX_RATE = 0.98

// ===== 定时检查 =====

/** 恢复检查间隔（毫秒），每小时检查一次 */
export const RECOVERY_CHECK_INTERVAL_MS = 60 * 60 * 1000

// ===== 持久化 =====

/** localStorage 存储 key */
export const STATUS_STORAGE_KEY = 'animal_poke_status'

// ===== 状态展示元数据 =====

/**
 * 状态展示元数据表
 */
export const STATUS_META: Record<StatusType, {
  label: string
  emoji: string
  color: string
  description: string
}> = {
  normal: {
    label: '正常',
    emoji: '✅',
    color: 'var(--success)',
    description: '状态良好',
  },
  cold: {
    label: '感冒',
    emoji: '🤧',
    color: 'var(--warn)',
    description: '全属性 -35%',
  },
  pleasure: {
    label: '愉悦',
    emoji: '😊',
    color: 'var(--orange)',
    description: '全属性 +5%（晴天）',
  },
}
