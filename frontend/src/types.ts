import {
  buildSpeciesDefs,
  capturableSpeciesIds,
  getRarityWeights,
  getSpeciesPack,
  getSpeciesDef,
  isCapturableSpecies,
} from './species'
import type { SpeciesDef as PackSpeciesDef } from './species'

export type RarityTier = 'common' | 'uncommon' | 'rare' | 'epic' | 'legendary'

// ===== 物种系统（Issue #35 / AP-093 内容包） =====

/** 物种内容 ID（扩展时追加 pack，无需改业务 switch） */
export type SpeciesType = string
export const UNKNOWN_SPECIES: SpeciesType = 'unknown'

/** 物种定义（由内容包投影） */
export type SpeciesDef = PackSpeciesDef

/** 物种定义表 — 来自内容注册表 */
export const SPECIES_DEFS: Record<string, SpeciesDef> = buildSpeciesDefs()

/** 当前可捕获物种（已 recognition 认证） */
export const CAPTURABLE_SPECIES: readonly string[] = capturableSpeciesIds()

/** 物种稀有度权重（内容驱动） */
export const SPECIES_RARITY_WEIGHTS: Record<string, { tier: RarityTier; weight: number }[]> =
  Object.fromEntries(
    capturableSpeciesIds().map((id) => {
      const weights = getRarityWeights(id).map((w) => ({
        tier: w.tier as RarityTier,
        weight: w.weight,
      }))
      return [id, weights]
    }),
  )

export interface CardEntry {
  id: string
  no: string
  rarity: RarityTier
  /** 物种内容 ID（可选，兼容旧数据） */
  species?: SpeciesType
  /** 内容包版本（可选） */
  speciesVersion?: string
  unlocked: boolean
  captureDate: string
  location: string
  lat: number
  lng: number
  seed: number
  isNew?: boolean
}

/** 将读取到的物种收敛为注册 ID；缺失或未知 ID 只用于安全展示。 */
export function normalizeSpeciesId(value: unknown): SpeciesType {
  const species = typeof value === 'string' ? value.trim() : ''
  return species && getSpeciesPack(species) ? species : UNKNOWN_SPECIES
}

/** 获取 CardEntry 的物种，旧数据缺失或未知时不得伪装成某个真实物种。 */
export function getCardSpecies(entry: CardEntry): SpeciesType {
  return normalizeSpeciesId(entry.species)
}

/** 是否允许捕获/发奖 */
export function canCaptureSpecies(species: string): boolean {
  return isCapturableSpecies(species)
}

/** 安全取物种定义（未知 ID 降级） */
export function resolveSpeciesDef(species: string): SpeciesDef {
  return SPECIES_DEFS[species] ?? getSpeciesDef(species)
}

export type FilterTab = 'all' | 'today' | 'week' | 'nearby'
export type MainTab = 'profile' | 'collection' | 'camera' | 'fight' | 'store' | 'dispatch' | 'achievement'

export const RARITY_COLORS: Record<RarityTier, string> = {
  common: 'var(--rarity-common)',
  uncommon: 'var(--rarity-uncommon)',
  rare: 'var(--rarity-rare)',
  epic: 'var(--rarity-epic)',
  legendary: 'var(--rarity-legendary)',
}

export const RARITY_NAMES: Record<RarityTier, string> = {
  common: '普通',
  uncommon: '少见',
  rare: '稀有',
  epic: '史诗',
  legendary: '传说',
}

export const MOCK_ENTRIES: CardEntry[] = [
  { id: 'c001', no: '#000059', rarity: 'common', species: 'cat', unlocked: true, captureDate: '2026-07-08', location: '海曙区·月湖', lat: 29.87, lng: 121.55, seed: 1, isNew: true },
  { id: 'c002', no: '#000058', rarity: 'uncommon', species: 'goose', unlocked: true, captureDate: '2026-07-07', location: '鄞州区·公园', lat: 29.83, lng: 121.57, seed: 2, isNew: true },
  { id: 'c003', no: '#000057', rarity: 'rare', species: 'dog', unlocked: true, captureDate: '2026-07-06', location: '江北区·滨江', lat: 29.90, lng: 121.53, seed: 3 },
  { id: 'c004', no: '#000056', rarity: 'common', species: 'cat', unlocked: true, captureDate: '2026-07-05', location: '海曙区·老街', lat: 29.86, lng: 121.54, seed: 4 },
  { id: 'c005', no: '#000055', rarity: 'epic', species: 'goose', unlocked: true, captureDate: '2026-07-04', location: '镇海区·山林', lat: 29.95, lng: 121.60, seed: 5 },
  { id: 'c006', no: '#000054', rarity: 'common', species: 'dog', unlocked: true, captureDate: '2026-07-03', location: '鄞州区·广场', lat: 29.82, lng: 121.58, seed: 6 },
  { id: 'c007', no: '#000014', rarity: 'legendary', species: 'cat', unlocked: true, captureDate: '2026-07-01', location: '海曙区·月湖', lat: 29.87, lng: 121.55, seed: 7 },
  { id: 'c008', no: '#???', rarity: 'common', species: 'goose', unlocked: false, captureDate: '—', location: '待发现', lat: 0, lng: 0, seed: 8 },
  { id: 'c009', no: '#???', rarity: 'common', species: 'dog', unlocked: false, captureDate: '—', location: '待发现', lat: 0, lng: 0, seed: 9 },
  // 尚未发现的注册物种示例
  { id: 'c010', no: '#E001', rarity: 'common', species: 'rabbit', unlocked: false, captureDate: '—', location: '百科预告', lat: 0, lng: 0, seed: 10 },
]
