export type RarityTier = 'common' | 'uncommon' | 'rare' | 'epic' | 'legendary'

// ===== 物种系统（Issue #35） =====

/** 物种标识 */
export type SpeciesType = 'cat' | 'goose' | 'dog'

/** 物种定义 */
export interface SpeciesDef {
  /** 物种标识 */
  species: SpeciesType
  /** 中文名 */
  name: string
  /** 显示 emoji */
  emoji: string
  /** 投掷物中文名 */
  throwItem: string
  /** 投掷物 emoji */
  throwItemEmoji: string
  /** 手感描述（仅用于 UI 提示） */
  captureMechanics: string
  /** 充能速率（每 tick 增加百分比） */
  chargeRate: number
  /** 最佳力度区间 [min, max]（百分比） */
  optimalRange: [number, number]
}

/** 物种定义表 */
export const SPECIES_DEFS: Record<SpeciesType, SpeciesDef> = {
  cat: {
    species: 'cat',
    name: '猫',
    emoji: '🐱',
    throwItem: '猫粮罐',
    throwItemEmoji: '🥫',
    captureMechanics: '标准抛物线',
    chargeRate: 2,
    optimalRange: [40, 80],
  },
  goose: {
    species: 'goose',
    name: '鹅',
    emoji: '🪿',
    throwItem: '面包屑球',
    throwItemEmoji: '🍞',
    captureMechanics: '弹跳略强',
    chargeRate: 2.5,
    optimalRange: [35, 75],
  },
  dog: {
    species: 'dog',
    name: '狗',
    emoji: '🐶',
    throwItem: '骨头零食',
    throwItemEmoji: '🦴',
    captureMechanics: '下落更快',
    chargeRate: 1.5,
    optimalRange: [45, 85],
  },
}

/** 物种稀有度权重（猫偏 common，鹅偏 uncommon，狗 balanced） */
export const SPECIES_RARITY_WEIGHTS: Record<SpeciesType, { tier: RarityTier; weight: number }[]> = {
  cat: [
    { tier: 'common', weight: 70 },
    { tier: 'uncommon', weight: 25 },
    { tier: 'rare', weight: 12 },
    { tier: 'epic', weight: 3 },
    { tier: 'legendary', weight: 1 },
  ],
  goose: [
    { tier: 'common', weight: 45 },
    { tier: 'uncommon', weight: 35 },
    { tier: 'rare', weight: 20 },
    { tier: 'epic', weight: 7 },
    { tier: 'legendary', weight: 3 },
  ],
  dog: [
    { tier: 'common', weight: 55 },
    { tier: 'uncommon', weight: 30 },
    { tier: 'rare', weight: 15 },
    { tier: 'epic', weight: 5 },
    { tier: 'legendary', weight: 2 },
  ],
}

export interface CardEntry {
  id: string
  no: string
  rarity: RarityTier
  /** 物种标识（可选，兼容旧数据） */
  species?: SpeciesType
  unlocked: boolean
  captureDate: string
  location: string
  lat: number
  lng: number
  seed: number
  isNew?: boolean
}

/** 获取 CardEntry 的物种，未设置的旧数据默认猫 */
export function getCardSpecies(entry: CardEntry): SpeciesType {
  return entry.species ?? 'cat'
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
]
