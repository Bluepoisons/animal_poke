/** AP-133 city anthology packs + difference / privacy checks. */

export interface CityRole {
  id: string
  perspective: string
}

export interface CityNode {
  id: string
  conflict: string
  homeModeAlt: string
}

export interface CityPack {
  id: string
  displayName: string
  themeConflict: string
  spatialTexture: string
  seasonRhythm: string
  roles: CityRole[]
  nodes: CityNode[]
  /** Coarse place labels only */
  placeLabels: string[]
  /** Private/sensitive deny list patterns */
  placeBlacklist: string[]
  assetLicenseIds: string[]
  /** When local assets missing */
  syntheticFallbackRegion: string
}

export const cityHarbor: CityPack = {
  id: 'city.harbor_a',
  displayName: '港湾城（试点 A）',
  themeConflict: '灯光治理 vs 夜行生境',
  spatialTexture: '堤岸—闸门—步道灯带',
  seasonRhythm: '梅雨季潮位叙事',
  roles: [
    { id: 'guide', perspective: '本地向导' },
    { id: 'volunteer', perspective: '护河志愿者' },
  ],
  nodes: [
    { id: 'h1', conflict: '围挡占坡道', homeModeAlt: '热线记录整理' },
    { id: 'h2', conflict: '灯带过亮', homeModeAlt: '回忆便签观灯' },
  ],
  placeLabels: ['滨水区', '观景台', '堤岸步道'],
  placeBlacklist: ['门牌', '小区名', '学校全名', '私人住址'],
  assetLicenseIds: ['lic.self.river_ambience_v1'],
  syntheticFallbackRegion: '虚构港湾合成区',
}

export const cityInland: CityPack = {
  id: 'city.inland_b',
  displayName: '内陆城（试点 B）',
  themeConflict: '旱季渠道养护 vs 街头树荫公共性',
  spatialTexture: '渠道—桥荫—旧书场',
  seasonRhythm: '秋旱与夜市降温',
  roles: [
    { id: 'archivist', perspective: '渠道档案员' },
    { id: 'vendor', perspective: '树荫摊主（虚构）' },
  ],
  nodes: [
    { id: 'i1', conflict: '渠道清淤时段与通学冲突', homeModeAlt: '档案复印件阅读' },
    { id: 'i2', conflict: '树荫被临时摊位挤占', homeModeAlt: '市民留言板整理' },
  ],
  placeLabels: ['渠道段', '桥荫广场', '旧书场'],
  placeBlacklist: ['门牌', '小区名', '学校全名', '私人住址'],
  assetLicenseIds: ['lic.self.guide_open_v1'],
  syntheticFallbackRegion: '虚构内陆合成区',
}

export function listCityPacks(): CityPack[] {
  return [cityHarbor, cityInland]
}

/** Structural difference score: roles/conflicts/nodes should not be isomorphic copy. */
export function differenceReport(a: CityPack, b: CityPack): {
  ok: boolean
  sharedRoleIds: string[]
  sharedNodeConflicts: string[]
  themeSame: boolean
} {
  const rolesA = new Set(a.roles.map((r) => r.id))
  const sharedRoleIds = b.roles.map((r) => r.id).filter((id) => rolesA.has(id))
  const confA = new Set(a.nodes.map((n) => n.conflict))
  const sharedNodeConflicts = b.nodes.map((n) => n.conflict).filter((c) => confA.has(c))
  const themeSame = a.themeConflict === b.themeConflict
  const ok = sharedRoleIds.length === 0 && sharedNodeConflicts.length === 0 && !themeSame
  return { ok, sharedRoleIds, sharedNodeConflicts, themeSame }
}

export function placeIsSafe(pack: CityPack, label: string): boolean {
  const lower = label.toLowerCase()
  for (const bad of pack.placeBlacklist) {
    if (lower.includes(bad.toLowerCase())) return false
  }
  // reject street numbers looking private
  if (/\d{1,4}号/.test(label)) return false
  return pack.placeLabels.includes(label) || label === pack.syntheticFallbackRegion
}

export function resolveRegionLabel(pack: CityPack, preferred: string | undefined, hasLocalAssets: boolean): string {
  if (!hasLocalAssets) return pack.syntheticFallbackRegion
  if (preferred && placeIsSafe(pack, preferred)) return preferred
  return pack.placeLabels[0] ?? pack.syntheticFallbackRegion
}
