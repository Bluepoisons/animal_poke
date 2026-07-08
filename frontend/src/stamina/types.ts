/** 等级表单行结构 */
export interface LevelTableRow {
  level: number
  /** 升级所需累计捕获数 */
  requiredCaptures: number
  /** 该等级体力上限 */
  maxStamina: number
  /** 升级奖励金币数（Lv.1 无升级奖励，为 0） */
  rewardGold: number
  /** 是否为满级 */
  isMaxLevel: boolean
}

/** 体力系统完整状态 */
export interface StaminaState {
  /** 当前等级（1~10） */
  level: number
  /** 当前体力值（0 ~ maxStamina） */
  currentStamina: number
  /** 累计捕获数（用于判定升级） */
  totalCaptures: number
  /** 上次自然恢复结算的时间戳（Unix ms） */
  lastRecoverTime: number
  /** 当前金币数 */
  gold: number
  /** 今日已购买体力药剂次数（0~3） */
  potionPurchasesToday: number
  /** 今日购买计数的日期标记（自然日，格式 'YYYY-MM-DD'） */
  potionPurchaseDate: string
}

/** 升级结果 */
export interface LevelUpResult {
  /** 是否发生了升级 */
  leveledUp: boolean
  /** 升级后的新等级（未升级则为原等级） */
  newLevel: number
  /** 升级奖励金币数（未升级则为 0） */
  rewardGold: number
}

/** 自然恢复计算结果 */
export interface RecoveryResult {
  /** 恢复后的体力值 */
  current: number
  /** 距离下次恢复还需多少秒 */
  recoverTime: number
}

/** 购买体力药剂结果 */
export interface BuyPotionResult {
  /** 是否购买成功 */
  success: boolean
  /** 今日剩余购买次数 */
  remainingPurchases: number
  /** 失败原因（success=false 时有值） */
  reason?: 'insufficient_gold' | 'daily_limit_reached'
}

/** Reducer Action 类型 */
export type StaminaAction =
  | { type: 'TICK_RECOVERY'; now: number }
  | { type: 'CONSUME'; amount: number }
  | { type: 'ADD_STAMINA'; amount: number }
  | { type: 'ADD_CAPTURE'; count: number }
  | { type: 'ADD_GOLD'; amount: number }
  | { type: 'BUY_POTION' }
  | { type: 'RESET_DAILY_PURCHASES'; date: string }
  | { type: 'LOAD_STATE'; state: StaminaState }

/** StaminaContext 暴露给组件的接口 */
export interface StaminaContextValue {
  state: StaminaState
  /** 当前等级对应的体力上限 */
  maxStamina: number
  /** 距离下次恢复的秒数（用于 UI 倒计时显示） */
  nextRecoverIn: number
  /** 消耗体力，返回是否成功（体力不足返回 false） */
  consumeStamina: (amount: number) => boolean
  /** 增加体力（不超过上限） */
  addStamina: (amount: number) => void
  /** 增加捕获数，内部自动检查升级 */
  addCapture: (count: number) => LevelUpResult
  /** 增加金币 */
  addGold: (amount: number) => void
  /** 购买体力药剂（150 金币换 3 体力，每日限 3 次） */
  buyStaminaPotion: () => BuyPotionResult
}
