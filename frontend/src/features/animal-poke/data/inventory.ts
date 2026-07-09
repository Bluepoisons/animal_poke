import type { InventoryItem } from './types'

export const inventoryItems: InventoryItem[] = [
  {
    id: 'toy-ball',
    icon: 'ball',
    name: '玩具球',
    effect: '捕获 +15%',
    price: 50,
  },
  {
    id: 'advanced-ball',
    icon: 'super-ball',
    name: '高级玩具球',
    effect: '捕获 +25%',
    price: 120,
  },
  {
    id: 'bait',
    icon: 'bait',
    name: '诱饵',
    effect: '30 分钟稀有提升',
    price: 100,
  },
  {
    id: 'energy-potion',
    icon: 'potion',
    name: '体力药剂',
    effect: '体力 +3',
    price: 150,
  },
]
