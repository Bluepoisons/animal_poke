/**
 * 物种内容注册表（AP-093）
 * 业务通过 ID/版本/状态查询，避免 cat|dog|goose 多处 switch。
 */
import { SPECIES_PACKS } from './packs'
import type {
  RecognitionStatus,
  SpeciesDef,
  SpeciesPack,
  SpeciesRef,
  SpeciesGroup,
  SpeciesRarityWeight,
  SpeciesStatModifiers,
} from './types'
import { localizedOr } from './types'

const DEFAULT_STAT_MOD: SpeciesStatModifiers = {
  hp: 1,
  atk: 1,
  def: 1,
  spd: 1,
  crit: 0,
  eva: 0,
}

function major(v: string): string {
  const s = (v || '').trim()
  if (!s) return ''
  const i = s.indexOf('.')
  return i >= 0 ? s.slice(0, i) : s
}

function compatibleVersion(cert: string, expected: string): boolean {
  const cm = major(cert)
  const em = major(expected)
  if (!cm || !em) return cert === expected
  return cm === em
}

function hasCapturableGameplay(p: SpeciesPack): boolean {
  const g = p.gameplay
  if (!g) return false
  if (!g.detectThreshold || g.detectThreshold <= 0 || g.detectThreshold > 1) return false
  if (!g.statModifiers) return false
  const sm = g.statModifiers
  if (sm.hp <= 0 || sm.atk <= 0 || sm.def <= 0 || sm.spd <= 0) return false
  if (!g.rarityWeights || g.rarityWeights.length === 0) return false
  return true
}

/** 运行时有效状态（认证过期 / 缺字段安全降级） */
export function effectiveStatus(
  pack: SpeciesPack,
  now: Date = new Date(),
  expectedGoldenVersion = '',
): RecognitionStatus {
  if (!pack?.id || !pack.version || !pack.contentId) return 'catalog_only'
  if (!localizedOr(pack.names?.common) || !pack.assets?.emoji) return 'catalog_only'

  if (pack.status === 'catalog_only') return 'catalog_only'
  if (pack.status !== 'capturable' && pack.status !== 'recognition_certified') {
    return 'catalog_only'
  }

  const cert = pack.certification
  if (!cert?.goldenSetVersion) return 'catalog_only'
  if (cert.expiresAt) {
    const exp = new Date(cert.expiresAt)
    if (!Number.isNaN(exp.getTime()) && now.getTime() > exp.getTime()) {
      return 'catalog_only'
    }
  }
  if (expectedGoldenVersion && !compatibleVersion(cert.goldenSetVersion, expectedGoldenVersion)) {
    return 'catalog_only'
  }

  if (pack.status === 'capturable' && !hasCapturableGameplay(pack)) {
    return 'recognition_certified'
  }
  return pack.status
}

export function isCapturable(
  pack: SpeciesPack,
  now?: Date,
  expectedGoldenVersion?: string,
): boolean {
  return effectiveStatus(pack, now, expectedGoldenVersion) === 'capturable'
}

class SpeciesRegistry {
  private packs = new Map<string, SpeciesPack>()

  constructor(initial: SpeciesPack[] = SPECIES_PACKS) {
    for (const p of initial) {
      this.packs.set(p.id, p)
    }
  }

  get(id: string): SpeciesPack | undefined {
    return this.packs.get(id)
  }

  all(): SpeciesPack[] {
    return [...this.packs.values()]
  }

  encyclopediaIds(): string[] {
    return this.all()
      .map((p) => p.id)
      .sort()
  }

  capturableIds(now?: Date, expectedGoldenVersion?: string): string[] {
    // 保持内容包注册顺序（非字典序），避免破坏掉率/随机池约定
    return this.all()
      .filter((p) => isCapturable(p, now, expectedGoldenVersion))
      .map((p) => p.id)
  }

  canCapture(id: string, now?: Date, expectedGoldenVersion?: string): boolean {
    const p = this.packs.get(id)
    if (!p) return false
    return isCapturable(p, now, expectedGoldenVersion)
  }

  statusOf(id: string, now?: Date, expectedGoldenVersion?: string): RecognitionStatus | '' {
    const p = this.packs.get(id)
    if (!p) return ''
    return effectiveStatus(p, now, expectedGoldenVersion)
  }

  ref(id: string): SpeciesRef {
    const p = this.packs.get(id)
    return { id, version: p?.version ?? '' }
  }

  /** 注册/覆盖（测试或热更新） */
  register(pack: SpeciesPack): void {
    this.packs.set(pack.id, pack)
  }

  /** 移除（测试清理） */
  unregister(id: string): void {
    this.packs.delete(id)
  }
}

export const speciesRegistry = new SpeciesRegistry()
export function getSpeciesPack(id: string): SpeciesPack | undefined {
  return speciesRegistry.get(id)
}

export function capturableSpeciesIds(): string[] {
  return speciesRegistry.capturableIds()
}

export function encyclopediaSpeciesIds(): string[] {
  return speciesRegistry.encyclopediaIds()
}

export function isCapturableSpecies(id: string): boolean {
  return speciesRegistry.canCapture(id)
}

export function speciesGroupOf(id: string): SpeciesGroup {
  return speciesRegistry.get(id)?.group ?? 'other'
}

function normalizeLabel(value: string): string {
  return value
    .trim()
    .toLowerCase()
    .replace(/[_-]+/g, ' ')
    .replace(/[^\p{L}\p{N}]+/gu, ' ')
    .replace(/\s+/g, ' ')
    .trim()
}

function normalizedTerms(pack: SpeciesPack): string[] {
  return [
    pack.id,
    ...Object.values(pack.names.common),
    pack.names.scientific ?? '',
    ...(pack.names.aliases ?? []),
    ...(pack.names.contains ?? []),
  ]
    .map(normalizeLabel)
    .filter(Boolean)
}

function containsLabelTerm(value: string, term: string): boolean {
  if (!value || !term) return false
  const chars = Array.from(term)
  if (chars.length === 1 && /\p{Script=Han}/u.test(chars[0])) return false
  if (/\p{Script=Han}/u.test(term)) return value.includes(term)
  return ` ${value} `.includes(` ${term} `)
}

/**
 * 使用内容包中的 ID、各语言名称、aliases 与 contains 解析模型标签。
 * 精确匹配优先；模糊匹配选择最长命中，避免通用 bird/fish 抢走具体物种。
 */
export function findSpeciesIdByLabel(raw?: string, capturableOnly = true): string | null {
  const value = normalizeLabel(raw ?? '')
  if (!value) return null

  const packs = speciesRegistry
    .all()
    .filter((pack) => !capturableOnly || isCapturable(pack))

  for (const pack of packs) {
    if (normalizedTerms(pack).includes(value)) return pack.id
  }

  let best: { id: string; score: number } | null = null
  for (const pack of packs) {
    const excludes = (pack.names.containsExclude ?? []).map(normalizeLabel).filter(Boolean)
    if (excludes.some((term) => containsLabelTerm(value, term))) continue

    for (const rawTerm of pack.names.contains ?? []) {
      const term = normalizeLabel(rawTerm)
      if (!containsLabelTerm(value, term)) continue
      const genericPenalty = pack.id === 'bird' || pack.id === 'fish' ? 1 : 0
      const score = Array.from(term).length * 10 - genericPenalty
      if (!best || score > best.score) best = { id: pack.id, score }
    }
  }
  return best?.id ?? null
}

export function speciesContentRef(id: string): SpeciesRef {
  return speciesRegistry.ref(id)
}

/** 转为 UI SpeciesDef（缺字段安全默认） */
export function toSpeciesDef(pack: SpeciesPack, locale = 'zh-CN'): SpeciesDef {
  const g = pack.gameplay
  const range = g?.optimalRange
  return {
    species: pack.id,
    name: localizedOr(pack.names.common, locale) || pack.id,
    emoji: pack.assets.emoji || '❓',
    throwItem: localizedOr(g?.throwItem, locale) || '观察道具',
    throwItemEmoji: pack.assets.throwItemEmoji || '✨',
    captureMechanics: localizedOr(g?.captureMechanics, locale) || '标准',
    chargeRate: g?.chargeRate ?? 2,
    optimalRange: range && range.length === 2 ? [range[0], range[1]] : [40, 80],
    status: effectiveStatus(pack),
    contentId: pack.contentId,
    version: pack.version,
    detectThreshold: g?.detectThreshold ?? 0.85,
  }
}

/** SPECIES_DEFS 兼容视图：含百科物种 */
export function buildSpeciesDefs(locale = 'zh-CN'): Record<string, SpeciesDef> {
  const out: Record<string, SpeciesDef> = {}
  for (const p of speciesRegistry.all()) {
    out[p.id] = toSpeciesDef(p, locale)
  }
  return out
}

export function getSpeciesDef(id: string, locale = 'zh-CN'): SpeciesDef {
  const p = speciesRegistry.get(id)
  if (p) return toSpeciesDef(p, locale)
  // 安全降级：未知内容 ID
  return {
    species: id || 'unknown',
    name: '动物伙伴',
    emoji: '❓',
    throwItem: '观察道具',
    throwItemEmoji: '✨',
    captureMechanics: '标准',
    chargeRate: 2,
    optimalRange: [40, 80],
    status: 'catalog_only',
    contentId: id ? `species.${id}` : 'species.unknown',
    version: '',
    detectThreshold: 0.85,
  }
}

export function getStatModifiers(id: string): SpeciesStatModifiers {
  const p = speciesRegistry.get(id)
  return p?.gameplay?.statModifiers ?? DEFAULT_STAT_MOD
}

export function getRarityWeights(id: string): SpeciesRarityWeight[] {
  const p = speciesRegistry.get(id)
  return p?.gameplay?.rarityWeights ?? [
    { tier: 'common', weight: 60 },
    { tier: 'uncommon', weight: 25 },
    { tier: 'rare', weight: 10 },
    { tier: 'epic', weight: 4 },
    { tier: 'legendary', weight: 1 },
  ]
}

export function getDetectThreshold(id: string): number {
  return getSpeciesDef(id).detectThreshold
}

export function getChargeSpeed(id: string): number {
  return speciesRegistry.get(id)?.gameplay?.chargeSpeed ?? 1
}

/** schema 轻量校验（测试用） */
export function validatePackSchema(pack: SpeciesPack): string[] {
  const errors: string[] = []
  if (!pack.id) errors.push('id required')
  if (pack.id === 'unknown' || pack.id === 'unsupported') errors.push('id reserved')
  if (!pack.version) errors.push('version required')
  if (!pack.contentId) errors.push('content_id required')
  if (!['catalog_only', 'recognition_certified', 'capturable'].includes(pack.status)) {
    errors.push('invalid status')
  }
  if (!localizedOr(pack.names?.common)) errors.push('names.common required')
  if (!pack.assets?.emoji) errors.push('assets.emoji required')
  if (!pack.welfare?.level) errors.push('welfare.level required')
  if (!pack.protection?.status) errors.push('protection.status required')
  if (pack.status === 'capturable' || pack.status === 'recognition_certified') {
    if (!pack.certification?.goldenSetVersion) errors.push('certification required')
  }
  if (pack.status === 'capturable' && !hasCapturableGameplay(pack)) {
    errors.push('capturable gameplay incomplete')
  }
  return errors
}
