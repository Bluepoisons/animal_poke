/**
 * LBS 发现点 → 猎取地图投影（#170）
 */
import type { DiscoveryPoint, GeoLocation } from '../../lbs/types'
import type { HuntTarget, Rarity, Species } from './data/types'
import type { RarityTier } from '../../types'

const RARITY_MAP: Record<RarityTier, Rarity> = {
  common: 'common',
  uncommon: 'uncommon',
  rare: 'rare',
  epic: 'epic',
  legendary: 'legendary',
}

/** 将相对米偏移投影到 0.08~0.92 画布坐标 */
export function projectToMap(
  player: GeoLocation,
  point: { lat: number; lng: number },
  scaleMeters = 600,
): { x: number; y: number; distanceMeters: number } {
  // 粗略：1 deg lat ≈ 111320 m；lng 随纬度缩放
  const mPerDegLat = 111_320
  const mPerDegLng = 111_320 * Math.cos((player.lat * Math.PI) / 180)
  const dLatM = (point.lat - player.lat) * mPerDegLat
  const dLngM = (point.lng - player.lng) * mPerDegLng
  const distanceMeters = Math.sqrt(dLatM * dLatM + dLngM * dLngM)
  const x = 0.5 + dLngM / scaleMeters / 2
  const y = 0.5 - dLatM / scaleMeters / 2
  return {
    x: Math.min(0.92, Math.max(0.08, x)),
    y: Math.min(0.92, Math.max(0.08, y)),
    distanceMeters: Math.round(distanceMeters),
  }
}

export function discoveryToHuntTarget(
  point: DiscoveryPoint,
  player: GeoLocation | null,
): HuntTarget {
  const proj = player
    ? projectToMap(player, point)
    : { x: 0.5, y: 0.5, distanceMeters: 0 }
  const species = point.species as Species
  return {
    id: point.id,
    species,
    rarity: RARITY_MAP[point.rarity] || 'common',
    distanceMeters: proj.distanceMeters,
    label: `${species} · ${proj.distanceMeters}m · ${point.status}`,
    x: proj.x,
    y: proj.y,
  }
}
