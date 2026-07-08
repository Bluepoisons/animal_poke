/**
 * 宠物状态类型
 * - normal:   默认正常状态
 * - cold:     感冒（全属性 -35%，持续 5 天）
 * - pleasure: 愉悦（全属性 +5%，晴天自动获得，不持久化）
 */
export type StatusType = 'normal' | 'cold' | 'pleasure'

/**
 * 状态触发来源
 * - weather: 天气触发（雨/雪天感冒）
 * - battle:  战斗触发
 * - capture: 捕获触发（新捕获宠物自带）
 * - item:    道具触发
 * - system:  系统自动（如定时恢复）
 */
export type StatusSource = 'weather' | 'battle' | 'capture' | 'item' | 'system'

/**
 * 一条状态效果实例
 */
export interface StatusEffect {
  /** 状态类型 */
  type: StatusType
  /** 触发来源 */
  source: StatusSource
  /** 生效时间戳（Unix ms） */
  startTime: number
  /** 持续天数 */
  durationDays: number
  /** 过期时间戳（Unix ms）= startTime + durationDays * DAY_MS */
  expiresAt: number
}

/**
 * 单只宠物的完整状态记录
 */
export interface PetStatusRecord {
  /** 宠物 ID（对应 CardEntry.id） */
  petId: string
  /** 当前活跃的状态效果列表（不含已过期） */
  effects: StatusEffect[]
  /**
   * 永久损伤倍率（累积值）
   * - 1.0 = 无损伤
   * - 0.95 = 一次永久损伤（-5%）
   * - 0.9025 = 两次永久损伤（-5% × 2）
   * 永久损伤下线时始终为 1.0
   */
  permanentDamageMultiplier: number
  /** 感冒历史次数（统计用，不影响逻辑） */
  coldCount: number
}

/**
 * 状态系统完整状态
 */
export interface StatusState {
  /** petId → 宠物状态记录 的映射 */
  records: Record<string, PetStatusRecord>
}

/**
 * Reducer Action 类型
 */
export type StatusAction =
  /** 为宠物施加感冒状态 */
  | { type: 'APPLY_COLD'; petId: string; source: StatusSource; now: number }
  /** 治愈宠物感冒（使用感冒药） */
  | { type: 'CURE_COLD'; petId: string }
  /** 为宠物施加愉悦状态 */
  | { type: 'APPLY_PLEASURE'; petId: string; now: number }
  /** 移除宠物愉悦状态 */
  | { type: 'REMOVE_PLEASURE'; petId: string }
  /** 检查并处理过期状态（自然恢复 + 永久损伤判定） */
  | { type: 'CHECK_RECOVERY'; now: number }
  /** 批量清理所有过期状态 */
  | { type: 'CLEAR_EXPIRED'; now: number }
  /** 移除某只宠物的所有状态（宠物被释放时） */
  | { type: 'CLEAR_PET'; petId: string }
  /** 加载持久化状态 */
  | { type: 'LOAD_STATE'; state: StatusState }

/**
 * 治疗结果
 */
export interface CureColdResult {
  success: boolean
  reason?: 'no_cold' | 'no_medicine'
}

/**
 * 感冒触发结果
 */
export interface ColdTriggerResult {
  triggered: boolean
  petId: string
}

/**
 * 状态展示信息（供 UI 使用）
 */
export interface StatusDisplay {
  /** 状态类型 */
  type: StatusType
  /** 中文名 */
  label: string
  /** emoji 图标 */
  emoji: string
  /** 颜色 CSS 变量名 */
  color: string
  /** 详细描述 */
  description: string
  /** 剩余天数（感冒用，愉悦为 null） */
  remainingDays?: number
}

/**
 * StatusContext 暴露给组件的接口
 */
export interface StatusContextValue {
  state: StatusState

  // ---- 查询函数 ----

  /** 获取宠物的所有活跃状态效果 */
  getPetEffects: (petId: string) => StatusEffect[]
  /** 获取宠物的状态展示信息列表（供 UI 渲染） */
  getPetStatusDisplay: (petId: string) => StatusDisplay[]
  /** 判断宠物是否有指定状态 */
  hasStatus: (petId: string, type: StatusType) => boolean
  /** 获取宠物的属性修正倍率（感冒 + 永久损伤，不含愉悦） */
  getStatModifier: (petId: string) => number
  /** 获取宠物永久损伤倍率 */
  getPermanentDamage: (petId: string) => number
  /** 获取感冒剩余天数 */
  getColdRemainingDays: (petId: string) => number | null

  // ---- 操作函数 ----

  /** 施加感冒状态 */
  applyCold: (petId: string, source: StatusSource) => void
  /** 治愈感冒（需有感冒药，内部调用 ShopContext.useItem） */
  cureCold: (petId: string) => CureColdResult
  /** 施加愉悦状态 */
  applyPleasure: (petId: string) => void
  /** 移除愉悦状态 */
  removePleasure: (petId: string) => void
  /** 检查并处理自然恢复（定时调用） */
  checkRecovery: () => void
}
