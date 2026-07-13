import type { SpeciesType, RarityTier } from '../types'
import { capturableSpeciesIds } from '../species'
import type { GeoLocation, DiscoveryPoint } from './types'
import {
  RARITY_SPAWN_RATES,
  DISCOVERY_TTL_MS,
  CAPTURE_RANGE_M,
  DISCOVERY_RANGE_M,
} from './constants'
import {
  canMarkInRange,
  generateSafeSpawnPosition,
  isAccuracyTooLow,
} from '../outdoorSafety/logic'
import { DEFAULT_UNSAFE_ZONES, type UnsafeZone } from '../outdoorSafety/constants'

/** 可捕获物种池（内容包） */
const SPECIES_POOL: SpeciesType[] = capturableSpeciesIds()

/** 按掉率表随机生成稀有度 */
export function rollRarity(rand: number = Math.random()): RarityTier {
  let acc = 0
  for (const tier of ['common', 'uncommon', 'rare', 'epic', 'legendary'] as RarityTier[]) {
    acc += RARITY_SPAWN_RATES[tier]
    if (rand < acc) return tier
  }
  return 'common'
}

/** 随机选择物种 */
export function rollSpecies(rand: number = Math.random()): SpeciesType {
  const idx = Math.min(SPECIES_POOL.length - 1, Math.max(0, Math.floor(rand * SPECIES_POOL.length)))
  return SPECIES_POOL[idx]
}

/** 在玩家附近生成随机坐标 */
export function generateRandomPosition(
  center: GeoLocation,
  minMeters: number = 50,
  maxMeters: number = 500,
  rand: number = Math.random(),
  angleRand: number = Math.random(),
): GeoLocation {
  const distance = minMeters + rand * (maxMeters - minMeters)
  const angle = angleRand * Math.PI * 2
  const latPerMeter = 1 / 111000
  const lngPerMeter = 1 / (111000 * Math.cos(center.lat * Math.PI / 180))
  const dLat = distance * Math.cos(angle) * latPerMeter
  const dLng = distance * Math.sin(angle) * lngPerMeter
  return { lat: center.lat + dLat, lng: center.lng + dLng }
}

/** 计算两点间距离（米），平面近似 */
export function calculateDistance(
  a: { lat: number; lng: number },
  b: { lat: number; lng: number },
): number {
  const latPerMeter = 1 / 111000
  const lngPerMeter = 1 / (111000 * Math.cos(a.lat * Math.PI / 180))
  const dLat = (a.lat - b.lat) / latPerMeter
  const dLng = (a.lng - b.lng) / lngPerMeter
  return Math.round(Math.hypot(dLat, dLng))
}

/** 将 GPS 坐标投影到画布百分比坐标 */
export function projectToCanvas(
  point: { lat: number; lng: number },
  center: { lat: number; lng: number },
  radiusMeters: number,
): { x: number; y: number } | null {
  const latPerMeter = 1 / 111000
  const lngPerMeter = 1 / (111000 * Math.cos(center.lat * Math.PI / 180))
  const dLatMeters = (point.lat - center.lat) / latPerMeter
  const dLngMeters = (point.lng - center.lng) / lngPerMeter
  const distMeters = Math.hypot(dLatMeters, dLngMeters)
  if (distMeters > radiusMeters) return null
  const x = 50 + (dLngMeters / radiusMeters) * 45
  const y = 50 - (dLatMeters / radiusMeters) * 45
  return { x, y }
}

/** 判断是否到了刷新时间 */
export function shouldRefresh(
  lastRefreshTime: number,
  now: number,
  intervalMs: number,
): boolean {
  return now - lastRefreshTime >= intervalMs
}

/** 过滤掉已过期的发现点 */
export function filterExpired(
  points: DiscoveryPoint[],
  now: number,
): DiscoveryPoint[] {
  return points.filter(p => p.expiresAt > now)
}

/**
 * 批量生成发现点（排除道路/水体/施工/私人/不可达启发式区域）
 */
export function generateDiscoveryPoints(
  center: GeoLocation,
  count: number,
  now: number,
  rand: () => number = Math.random,
  zones: UnsafeZone[] = DEFAULT_UNSAFE_ZONES,
): DiscoveryPoint[] {
  const points: DiscoveryPoint[] = []
  let attempts = 0
  const maxAttempts = count * 8
  while (points.length < count && attempts < maxAttempts) {
    attempts++
    const pos =
      generateSafeSpawnPosition(center, 50, DISCOVERY_RANGE_M, zones, rand) ??
      generateRandomPosition(center, 50, DISCOVERY_RANGE_M, rand(), rand())
    const unsafe = zones.some((z) => {
      const dist = calculateDistance({ lat: pos.lat, lng: pos.lng }, { lat: z.lat, lng: z.lng })
      return dist <= z.radiusM
    })
    if (unsafe) continue
    const rarity = rollRarity(rand())
    const species = rollSpecies(rand())
    points.push({
      id: createPointId(),
      lat: pos.lat,
      lng: pos.lng,
      species,
      rarity,
      spawnedAt: now,
      expiresAt: now + DISCOVERY_TTL_MS,
      status: 'available',
    })
  }
  return points
}

/**
 * 根据玩家位置更新发现点状态。
 * 精度超阈值时不允许 in_range（AP-045）。
 */
export function updatePointStatus(
  points: DiscoveryPoint[],
  playerLocation: GeoLocation,
  rangeMeters: number = CAPTURE_RANGE_M,
): DiscoveryPoint[] {
  const accuracy = playerLocation.accuracy
  const accuracyBlocks = isAccuracyTooLow(accuracy)
  return points.map(p => {
    const dist = calculateDistance({ lat: p.lat, lng: p.lng }, playerLocation)
    const inRange = canMarkInRange({
      distanceM: dist,
      captureRangeM: rangeMeters,
      accuracyM: accuracy,
    })
    return {
      ...p,
      status: inRange && !accuracyBlocks ? 'in_range' : 'available',
    }
  })
}

/** 生成发现点唯一 ID */
export function createPointId(): string {
  return `dp_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`
}
