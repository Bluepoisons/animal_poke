/**
 * AP-099: Researcher growth + pure virtual companion (client helpers).
 * Server is authoritative for XP / nodes; this module holds pure display logic
 * and API wrappers. No combat power, no decay, no paid power path.
 */

import { apiRequest } from '../../api/client'

export type GrowthTrackId = 'photography' | 'ecology' | 'safe_observation'

export type GrowthEventKind =
  | 'photo_capture'
  | 'photo_quality'
  | 'species_first'
  | 'species_research'
  | 'safe_explore'
  | 'distance_respect'
  | 'companion_interact'
  | 'companion_memory'
  | 'companion_decor'

export const GROWTH_CONFIG_VERSION = 'growth.v1'

/** Hard product rules — mirrored from server catalog */
export const GROWTH_RULES = {
  minVisibleNodesPerCompanion: 3,
  noDecay: true,
  noRealWorldFeeding: true,
  noPaidPower: true,
  combatStatsUnchanged: true,
  recognitionUnchanged: true,
  crossDeviceAuthoritative: true,
} as const

/** Forbidden client event kinds (must never be sent) */
export const FORBIDDEN_GROWTH_KINDS = [
  'feed',
  'real_feed',
  'decay',
  'affinity_decay',
  'paid_power',
  'iap_power',
  'combat_boost',
  'stat_boost',
  'battle_power',
] as const

export type ResearcherTrackView = {
  track: GrowthTrackId
  xp: number
  level: number
  config_version?: string
}

export type CompanionNodeView = {
  node_id: string
  title: string
  kind: 'memory' | 'decor' | 'journal' | string
  visible: boolean
  unlocked: boolean
  unlock_at_xp: number
}

export type CompanionProfileView = {
  animal_uuid: string
  bond_xp: number
  bond_level: number
  decor_stage: number
  title?: string
  config_version?: string
}

/** Level from cumulative XP thresholds (pure, matches server defaults) */
export function levelFromXP(xp: number, thresholds: number[]): number {
  const safe = Math.max(0, xp)
  let lvl = 0
  for (let i = 0; i < thresholds.length; i++) {
    if (safe >= thresholds[i]) lvl = i
  }
  return lvl
}

export const RESEARCHER_THRESHOLDS = [0, 20, 50, 100, 180, 300, 500, 800, 1200, 1800, 2500]
export const COMPANION_THRESHOLDS = [0, 10, 25, 50, 80, 120, 180, 260, 360, 500]

export function researcherLevel(xp: number): number {
  return levelFromXP(xp, RESEARCHER_THRESHOLDS)
}

export function companionLevel(xp: number): number {
  return levelFromXP(xp, COMPANION_THRESHOLDS)
}

/** Assert catalog / companion satisfies ≥3 visible nodes */
export function countVisibleNodes(nodes: Array<{ visible?: boolean }>): number {
  return nodes.filter((n) => n.visible !== false).length
}

export function hasMinVisibleNodes(nodes: Array<{ visible?: boolean }>, min = 3): boolean {
  return countVisibleNodes(nodes) >= min
}

export function isForbiddenGrowthKind(kind: string): boolean {
  return (FORBIDDEN_GROWTH_KINDS as readonly string[]).includes(kind)
}

/** Map capture / explore actions to growth event kinds (no combat) */
export function eventKindForAction(
  action: 'capture_first' | 'capture_repeat' | 'photo_quality' | 'safe_explore' | 'companion_interact',
): GrowthEventKind {
  switch (action) {
    case 'capture_first':
      return 'species_first'
    case 'capture_repeat':
      return 'species_research'
    case 'photo_quality':
      return 'photo_quality'
    case 'safe_explore':
      return 'safe_explore'
    case 'companion_interact':
      return 'companion_interact'
  }
}

// ---- API wrappers (server-authoritative) ----

export async function fetchGrowthCatalog(token: string) {
  return apiRequest<{
    config_version: string
    tracks: Array<{ id: string; title: string }>
    companion_nodes: Array<{ node_id: string; title: string; kind: string; unlock_at_xp: number }>
    rules: Record<string, unknown>
  }>({ method: 'GET', path: '/api/v1/growth/catalog', token })
}

export async function fetchResearcherGrowth(token: string) {
  return apiRequest<{
    owner_key: string
    config_version: string
    tracks: ResearcherTrackView[]
  }>({ method: 'GET', path: '/api/v1/growth/researcher', token })
}

export async function postGrowthEvent(
  token: string,
  body: {
    event_id: string
    kind: GrowthEventKind
    animal_uuid?: string
    source_type?: string
    source_id?: string
  },
) {
  if (isForbiddenGrowthKind(body.kind)) {
    throw new Error('growth_forbidden_kind')
  }
  return apiRequest<{
    idempotent: boolean
    combat_unchanged: boolean
    companion?: CompanionProfileView
    nodes?: CompanionNodeView[]
  }>({
    method: 'POST',
    path: '/api/v1/growth/events',
    token,
    body: JSON.stringify(body),
    idempotencyKey: body.event_id,
  })
}

export async function fetchCompanion(token: string, animalUUID: string) {
  return apiRequest<{
    companion: CompanionProfileView
    nodes: CompanionNodeView[]
    visible_nodes: number
    min_visible: number
    combat_unchanged: boolean
  }>({ method: 'GET', path: `/api/v1/growth/companions/${animalUUID}`, token })
}
