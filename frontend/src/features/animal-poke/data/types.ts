import type { RarityTier, SpeciesType } from '../../../types'

/**
 * Feature-local aliases for shared domain types (AP-032).
 * Canonical definitions live in `src/types.ts` and `src/shop/constants.ts`.
 */
export type Rarity = RarityTier
export type Species = SpeciesType

export type ScreenId =
  | 'discover'
  | 'map'
  | 'capture'
  | 'pokedex'
  | 'battle'
  | 'store'
  | 'settings'
  | 'journal'
  | 'prologue'

export type PokedexFilter = 'all' | 'cat' | 'goose' | 'dog'

export type Strategy = 'aggressive' | 'balanced' | 'defensive'

export interface AnimalEntry {
  id: string
  species: Species
  name: string
  rarity: Rarity
  collected: boolean
  region?: string
  location?: string
  trait?: string
  captureRate?: number
}

export interface HuntTarget {
  id: string
  species: Species
  rarity: Rarity
  distanceMeters: number
  label: string
  x: number
  y: number
}

export interface InventoryItem {
  id: string
  icon: 'ball' | 'super-ball' | 'bait' | 'potion'
  name: string
  effect: string
  price: number
  disabled?: boolean
}

export interface BattleState {
  playerSpecies: Species
  enemySpecies: Species
  playerHp: number
  enemyHp: number
  logLines: string[]
  activeStrategy: Strategy
}

export interface CheckInState {
  currentDay: number
  claimedToday: boolean
  rewards: number[]
}
