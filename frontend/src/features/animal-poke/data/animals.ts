import type { AnimalEntry, PokedexFilter } from './types'

export const animals: AnimalEntry[] = [
  {
    id: '000014',
    species: 'cat',
    name: '猫',
    rarity: 'legendary',
    collected: true,
    region: '海曙区',
    location: '月湖',
    trait: '标准抛物线',
    captureRate: 0.42,
  },
  {
    id: '000057',
    species: 'dog',
    name: '狗',
    rarity: 'rare',
    collected: true,
    region: '江北区',
    location: '滨江',
    trait: '下落更快',
    captureRate: 0.56,
  },
  {
    id: '000058',
    species: 'goose',
    name: '鹅',
    rarity: 'uncommon',
    collected: true,
    trait: '弹跳略强',
    captureRate: 0.78,
  },
  {
    id: 'locked-001',
    species: 'goose',
    name: '未知',
    rarity: 'common',
    collected: false,
  },
]

export const rarityNames: Record<AnimalEntry['rarity'], string> = {
  common: '普通',
  uncommon: '少见',
  rare: '稀有',
  epic: '史诗',
  legendary: '传说',
}

export function filterAnimals(
  entries: AnimalEntry[],
  filter: PokedexFilter,
): AnimalEntry[] {
  if (filter === 'all') return entries
  return entries.filter((entry) => entry.species === filter)
}

export const collectedCount = animals.filter((a) => a.collected).length
