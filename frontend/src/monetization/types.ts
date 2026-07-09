/** 商业化系统 — 双货币 + 月卡 + 通行证 */

export type CurrencyType = 'gold' | 'diamond'

export interface ProductBase {
  id: string
  name: string
  description: string
  priceDiamond: number
  icon: string
}

export interface ConsumableProduct extends ProductBase {
  type: 'consumable'
  effect: { type: 'gold' | 'stamina' | 'item'; amount: number }
}

export interface MonthlyCard extends ProductBase {
  type: 'monthly'
  dailyDiamond: number
  durationDays: number
  totalDiamond: number
}

export interface BattlePassSeason {
  seasonId: string
  name: string
  startDate: number
  endDate: number
  maxLevel: number
  freeTrack: BattlePassReward[]
  premiumTrack: BattlePassReward[]
  premiumPrice: number
}

export interface BattlePassReward {
  level: number
  type: 'gold' | 'diamond' | 'item' | 'cosmetic'
  amount: number
  name: string
}

export interface PurchaseResult {
  success: boolean
  productId: string
  currency: CurrencyType
  amount: number
  newBalance: number
  error?: string
}

/** 商店商品配置 */
export const CONSUMABLES: ConsumableProduct[] = [
  {
    id: 'gold_pack_s',
    name: '小金币包',
    description: '获得 1,000 金币',
    priceDiamond: 60,
    icon: '💰',
    type: 'consumable',
    effect: { type: 'gold', amount: 1000 },
  },
  {
    id: 'gold_pack_l',
    name: '大金币包',
    description: '获得 6,000 金币',
    priceDiamond: 300,
    icon: '💰',
    type: 'consumable',
    effect: { type: 'gold', amount: 6000 },
  },
  {
    id: 'stamina_potion',
    name: '体力药剂',
    description: '恢复 60 体力',
    priceDiamond: 50,
    icon: '⚡',
    type: 'consumable',
    effect: { type: 'stamina', amount: 60 },
  },
]

export const MONTHLY_CARD: MonthlyCard = {
  id: 'monthly_card',
  name: '月卡',
  description: '每天领取 60 钻石，持续 30 天',
  priceDiamond: 300,
  icon: '📅',
  type: 'monthly',
  dailyDiamond: 60,
  durationDays: 30,
  totalDiamond: 1800,
}

/** 计算月卡性价比 */
export function getMonthlyCardValue(card: MonthlyCard): number {
  return Math.round((card.totalDiamond / card.priceDiamond) * 100) / 100
}

/** 购买商品 */
export function purchase(
  productId: string,
  diamondBalance: number,
): PurchaseResult {
  const product = [...CONSUMABLES, MONTHLY_CARD].find(p => p.id === productId)
  if (!product) {
    return { success: false, productId, currency: 'diamond', amount: 0, newBalance: diamondBalance, error: '产品不存在' }
  }
  if (diamondBalance < product.priceDiamond) {
    return { success: false, productId, currency: 'diamond', amount: product.priceDiamond, newBalance: diamondBalance, error: '钻石不足' }
  }
  return {
    success: true,
    productId,
    currency: 'diamond',
    amount: product.priceDiamond,
    newBalance: diamondBalance - product.priceDiamond,
  }
}

/** 创建通行证赛季 mock */
export function createMockSeason(): BattlePassSeason {
  const now = Date.now()
  const seasonDuration = 56 * 24 * 60 * 60 * 1000 // 8 weeks
  return {
    seasonId: 'bp-s1',
    name: 'S1 起源赛季',
    startDate: now,
    endDate: now + seasonDuration,
    maxLevel: 50,
    premiumPrice: 680,
    freeTrack: Array.from({ length: 50 }, (_, i) => ({
      level: i + 1,
      type: 'gold' as const,
      amount: 100 * (i + 1),
      name: `${100 * (i + 1)} 金币`,
    })),
    premiumTrack: Array.from({ length: 50 }, (_, i) => ({
      level: i + 1,
      type: i % 10 === 9 ? 'cosmetic' : 'diamond',
      amount: i % 10 === 9 ? 1 : 50,
      name: i % 10 === 9 ? `限定皮肤 Lv${i + 1}` : `50 钻石`,
    })),
  }
}

/** 通行证进度 */
export function getBattlePassProgress(
  season: BattlePassSeason,
  currentExp: number,
  expPerLevel: number = 1000,
): { level: number; expIntoLevel: number; expForNextLevel: number } {
  const level = Math.min(season.maxLevel, Math.floor(currentExp / expPerLevel) + 1)
  const expIntoLevel = currentExp % expPerLevel
  return { level, expIntoLevel, expForNextLevel: expPerLevel }
}
