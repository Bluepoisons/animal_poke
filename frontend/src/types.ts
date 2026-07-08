export type RarityTier = 'common' | 'uncommon' | 'rare' | 'epic' | 'legendary'

export interface CardEntry {
  id: string
  no: string
  rarity: RarityTier
  unlocked: boolean
  captureDate: string
  location: string
  lat: number
  lng: number
  seed: number
  isNew?: boolean
}

export type FilterTab = 'all' | 'today' | 'week' | 'nearby'
export type MainTab = 'profile' | 'collection' | 'camera' | 'fight' | 'store'

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
  { id: 'c001', no: '#000059', rarity: 'common', unlocked: true, captureDate: '2026-07-08', location: '海曙区·月湖', lat: 29.87, lng: 121.55, seed: 1, isNew: true },
  { id: 'c002', no: '#000058', rarity: 'uncommon', unlocked: true, captureDate: '2026-07-07', location: '鄞州区·公园', lat: 29.83, lng: 121.57, seed: 2, isNew: true },
  { id: 'c003', no: '#000057', rarity: 'rare', unlocked: true, captureDate: '2026-07-06', location: '江北区·滨江', lat: 29.90, lng: 121.53, seed: 3 },
  { id: 'c004', no: '#000056', rarity: 'common', unlocked: true, captureDate: '2026-07-05', location: '海曙区·老街', lat: 29.86, lng: 121.54, seed: 4 },
  { id: 'c005', no: '#000055', rarity: 'epic', unlocked: true, captureDate: '2026-07-04', location: '镇海区·山林', lat: 29.95, lng: 121.60, seed: 5 },
  { id: 'c006', no: '#000054', rarity: 'common', unlocked: true, captureDate: '2026-07-03', location: '鄞州区·广场', lat: 29.82, lng: 121.58, seed: 6 },
  { id: 'c007', no: '#000014', rarity: 'legendary', unlocked: true, captureDate: '2026-07-01', location: '海曙区·月湖', lat: 29.87, lng: 121.55, seed: 7 },
  { id: 'c008', no: '#???', rarity: 'common', unlocked: false, captureDate: '—', location: '待发现', lat: 0, lng: 0, seed: 8 },
  { id: 'c009', no: '#???', rarity: 'common', unlocked: false, captureDate: '—', location: '待发现', lat: 0, lng: 0, seed: 9 },
]
