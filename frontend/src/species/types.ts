/**
 * 物种内容包类型（AP-093）
 * 与后端 backend/content/species 及 speciespack schema 对齐。
 */

export type RecognitionStatus =
  | 'catalog_only'
  | 'recognition_certified'
  | 'capturable'

export type SpeciesGroup =
  | 'companion'
  | 'farm'
  | 'wildlife'
  | 'bird'
  | 'reptile'
  | 'amphibian'
  | 'aquatic'
  | 'insect'
  | 'other'

export type Localized = Record<string, string>

export interface SpeciesCertification {
  goldenSetVersion: string
  modelTrack?: string
  certifiedAt?: string
  expiresAt?: string | null
}

export interface SpeciesWelfare {
  level: string
  notes?: Localized
}

export interface SpeciesProtection {
  status: string
  notes?: Localized
}

export interface SpeciesAssets {
  emoji: string
  icon?: string
  throwItemEmoji?: string
  placeholderTone?: string
}

export interface SpeciesStatModifiers {
  hp: number
  atk: number
  def: number
  spd: number
  crit: number
  eva: number
}

export interface SpeciesRarityWeight {
  tier: string
  weight: number
}

export interface SpeciesGameplay {
  throwItem?: Localized
  captureMechanics?: Localized
  chargeRate?: number
  optimalRange?: [number, number]
  chargeSpeed?: number
  detectThreshold?: number
  statModifiers?: SpeciesStatModifiers
  rarityWeights?: SpeciesRarityWeight[]
}

export interface SpeciesNames {
  common: Localized
  scientific?: string
  aliases?: string[]
  contains?: string[]
  containsExclude?: string[]
}

/** 物种内容包 */
export interface SpeciesPack {
  id: string
  /** Frontend browsing/correction category; unknown values safely fall back to other. */
  group?: SpeciesGroup
  version: string
  contentId: string
  status: RecognitionStatus
  certification?: SpeciesCertification
  names: SpeciesNames
  habitat?: Localized
  observationTips?: Localized
  welfare: SpeciesWelfare
  protection: SpeciesProtection
  assets: SpeciesAssets
  gameplay?: SpeciesGameplay
  i18n?: Record<string, Localized>
}

export interface SpeciesRef {
  id: string
  version: string
}

/** 兼容旧 UI 的物种定义视图 */
export interface SpeciesDef {
  species: string
  name: string
  emoji: string
  throwItem: string
  throwItemEmoji: string
  captureMechanics: string
  chargeRate: number
  optimalRange: [number, number]
  status: RecognitionStatus
  contentId: string
  version: string
  detectThreshold: number
}

export function localizedOr(m: Localized | undefined, locale = 'zh-CN'): string {
  if (!m) return ''
  if (locale && m[locale]) return m[locale]
  if (m['zh-CN']) return m['zh-CN']
  if (m.en) return m.en
  for (const v of Object.values(m)) {
    if (v) return v
  }
  return ''
}
