import type { HuntTarget } from './types'

export const huntTargets: HuntTarget[] = [
  {
    id: 'target-rare-230',
    species: 'dog',
    rarity: 'rare',
    distanceMeters: 230,
    label: '稀有 · 230m',
    x: 0.24,
    y: 0.37,
  },
  {
    id: 'target-legendary-480',
    species: 'cat',
    rarity: 'legendary',
    distanceMeters: 480,
    label: '传说 · 480m',
    x: 0.78,
    y: 0.3,
  },
  {
    id: 'target-uncommon-50',
    species: 'goose',
    rarity: 'uncommon',
    distanceMeters: 50,
    label: '发现点 · 50m 内可捕获',
    x: 0.72,
    y: 0.7,
  },
]

export function getTargetById(id: string): HuntTarget | undefined {
  return huntTargets.find((t) => t.id === id)
}
