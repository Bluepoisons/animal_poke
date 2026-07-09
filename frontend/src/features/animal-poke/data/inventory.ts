import type { InventoryItem } from './types'

export const inventoryItems: InventoryItem[] = [
  {
    id: 'toy-ball',
    icon: '🎾',
    name: '玩具球',
    effect: '捕获 +15%',
    price: 50,
  },
  {
    id: 'advanced-ball',
    icon: '⚾',
    name: '高级玩具球',
    effect: '捕获 +25%',
    price: 120,
  },
  {
    id: 'bait',
    icon: '🧀',
    name: '诱饵',
    effect: '30 分钟稀有提升',
    price: 100,
  },
  {
    id: 'energy-potion',
    icon: '🧪',
    name: '体力药剂',
    effect: '体力 +3',
    price: 150,
  },
]
