import type { ItemId } from '../../../shop/constants'
import { ITEM_DEFS } from '../../../shop/constants'
import type { InventoryItem } from './types'

/** UI 展示用商品列表 —— ID 必须与 ShopContext ItemId 一致 */
export const STORE_ITEM_IDS: ItemId[] = [
  'toy_ball',
  'premium_toy_ball',
  'bait',
  'stamina_potion',
]

const ICON_MAP: Record<ItemId, InventoryItem['icon']> = {
  toy_ball: 'ball',
  premium_toy_ball: 'super-ball',
  bait: 'bait',
  stamina_potion: 'potion',
  cold_medicine: 'potion',
  food_pack: 'bait',
}

export function toInventoryItem(id: ItemId): InventoryItem {
  const def = ITEM_DEFS[id]
  return {
    id,
    icon: ICON_MAP[id] || 'ball',
    name: def.name,
    effect: def.effect,
    price: def.price,
  }
}

export const inventoryItems: InventoryItem[] = STORE_ITEM_IDS.map(toInventoryItem)

/** 兼容旧 demo id → canonical ItemId */
export const LEGACY_ITEM_ID_MAP: Record<string, ItemId> = {
  'toy-ball': 'toy_ball',
  'advanced-ball': 'premium_toy_ball',
  'energy-potion': 'stamina_potion',
  bait: 'bait',
  toy_ball: 'toy_ball',
  premium_toy_ball: 'premium_toy_ball',
  stamina_potion: 'stamina_potion',
}
