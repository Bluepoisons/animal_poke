/**
 * 重复捕获价值（#210）
 * 首捕解锁图鉴；重复转化为研究点/亲密度
 */
import { chineseDetectedSpeciesName } from './petLocalization'

const KEY = 'animal-poke-collection-meta'

export interface SpeciesMeta {
  firstCaptureAt: number
  captureCount: number
  researchPoints: number
  affinity: number
}

export type CollectionMeta = Record<string, SpeciesMeta>

export function loadCollectionMeta(): CollectionMeta {
  try {
    const raw = localStorage.getItem(KEY)
    return raw ? (JSON.parse(raw) as CollectionMeta) : {}
  } catch {
    return {}
  }
}

export function saveCollectionMeta(m: CollectionMeta): void {
  try {
    localStorage.setItem(KEY, JSON.stringify(m))
  } catch {
    /* ignore */
  }
}

export type CaptureValueResult = {
  isFirst: boolean
  researchGained: number
  affinityGained: number
  message: string
  meta: SpeciesMeta
}

/** 登记一次捕获结果（按物种） */
function collectionSpeciesKey(species: string, speciesLabelZh?: string): string {
  const label = speciesLabelZh?.trim()
  return species === 'other_animal' && label ? `${species}:${label}` : species
}

export function registerCapture(species: string, speciesLabelZh?: string, now = Date.now()): CaptureValueResult {
  const all = loadCollectionMeta()
  const key = collectionSpeciesKey(species, speciesLabelZh)
  const prev = all[key]
  const speciesName = chineseDetectedSpeciesName(species, speciesLabelZh)
  if (!prev) {
    const meta: SpeciesMeta = {
      firstCaptureAt: now,
      captureCount: 1,
      researchPoints: 10,
      affinity: 5,
    }
    all[key] = meta
    saveCollectionMeta(all)
    return {
      isFirst: true,
      researchGained: 10,
      affinityGained: 5,
      message: `首次发现${speciesName}！动物记录已解锁`,
      meta,
    }
  }
  const researchGained = 3
  const affinityGained = 2
  const meta: SpeciesMeta = {
    ...prev,
    captureCount: prev.captureCount + 1,
    researchPoints: prev.researchPoints + researchGained,
    affinity: prev.affinity + affinityGained,
  }
  all[key] = meta
  saveCollectionMeta(all)
  return {
    isFirst: false,
    researchGained,
    affinityGained,
    message: `再次遇见${speciesName}：研究点 +${researchGained} · 亲密度 +${affinityGained}`,
    meta,
  }
}
