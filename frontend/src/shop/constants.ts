/** 道具 ID 枚举（字符串字面量联合类型） */
export type ItemId =
  | 'toy_ball'          // 玩具球
  | 'premium_toy_ball'  // 高级玩具球
  | 'cold_medicine'     // 感冒药
  | 'bait'              // 诱饵
  | 'stamina_potion'    // 体力药剂
  | 'food_pack'         // 食物包(猫)

/** 道具定义单行结构 */
export interface ItemDef {
  id: ItemId
  /** 道具名称（中文） */
  name: string
  /** 价格（金币） */
  price: number
  /** 图标 emoji */
  icon: string
  /** 道具描述 */
  description: string
  /** 道具效果说明（用于 UI 展示） */
  effect: string
  /** 道具类别 */
  category: 'capture' | 'recovery' | 'cure' | 'affinity'
  /** 是否单次使用即消耗 */
  consumable: boolean
  /** 每日限购次数（0 表示不限购） */
  dailyLimit: number
  /** 捕获增益百分比（仅 capture 类道具，0 表示无增益） */
  captureBoost: number
}

/** 道具定义表 */
export const ITEM_DEFS: Record<ItemId, ItemDef> = {
  toy_ball: {
    id: 'toy_ball',
    name: '玩具球',
    price: 50,
    icon: '🎾',
    description: '下次捕获时使用，提升捕获成功率',
    effect: '捕获成功率 +15%',
    category: 'capture',
    consumable: true,
    dailyLimit: 0,
    captureBoost: 15,
  },
  premium_toy_ball: {
    id: 'premium_toy_ball',
    name: '高级玩具球',
    price: 120,
    icon: '⚾',
    description: '下次捕获时使用，大幅提升捕获成功率',
    effect: '捕获成功率 +25%',
    category: 'capture',
    consumable: true,
    dailyLimit: 0,
    captureBoost: 25,
  },
  cold_medicine: {
    id: 'cold_medicine',
    name: '感冒药',
    price: 200,
    icon: '💊',
    description: '立即解除宠物的感冒状态',
    effect: '解除感冒',
    category: 'cure',
    consumable: true,
    dailyLimit: 0,
    captureBoost: 0,
  },
  bait: {
    id: 'bait',
    name: '诱饵',
    price: 100,
    icon: '🧀',
    description: '30 分钟内稀有动物出现率提升',
    effect: '稀有出现率提升（30 分钟）',
    category: 'capture',
    consumable: true,
    dailyLimit: 0,
    captureBoost: 0,
  },
  stamina_potion: {
    id: 'stamina_potion',
    name: '体力药剂',
    price: 150,
    icon: '🧪',
    description: '恢复 3 点体力',
    effect: '体力 +3',
    category: 'recovery',
    consumable: true,
    dailyLimit: 3,
    captureBoost: 0,
  },
  food_pack: {
    id: 'food_pack',
    name: '食物包(猫)',
    price: 30,
    icon: '🥫',
    description: '补充 10 个投掷物',
    effect: '投掷物 +10',
    category: 'capture',
    consumable: true,
    dailyLimit: 0,
    captureBoost: 0,
  },
}

/** 道具 ID 列表（用于遍历） */
export const ITEM_IDS = Object.keys(ITEM_DEFS) as ItemId[]

/** 签到奖励表（7 天递增，第 7 天满签额外送玩具球） */
export const CHECK_IN_REWARDS: number[] = [10, 20, 30, 50, 80, 120, 200]

/** 签到满签（第 7 天）额外赠送的道具 */
export const CHECK_IN_DAY7_BONUS_ITEM: ItemId = 'toy_ball'

/** 签到周期天数 */
export const CHECK_IN_CYCLE_DAYS = 7

/** 体力药剂恢复量 */
export const POTION_RECOVERY = 3

/** ShopContext localStorage 存储 key */
export const SHOP_STORAGE_KEY = 'animal_poke_shop'
