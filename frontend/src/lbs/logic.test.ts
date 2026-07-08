import { describe, it, expect } from 'vitest'
import {
  rollRarity,
  rollSpecies,
  generateRandomPosition,
  calculateDistance,
  projectToCanvas,
  shouldRefresh,
  filterExpired,
  generateDiscoveryPoints,
  updatePointStatus,
  createPointId,
} from './logic'
import { REFRESH_INTERVAL_MS, DISCOVERY_TTL_MS, CAPTURE_RANGE_M } from './constants'

// 测试用固定坐标（宁波市海曙区）
const NINGBO = { lat: 29.87, lng: 121.55 }

describe('rollRarity', () => {
  it('#1 rand=0.0 → common', () => {
    expect(rollRarity(0.0)).toBe('common')
  })

  it('#2 rand=0.59 → common (59% 仍在 common 60% 区间内)', () => {
    expect(rollRarity(0.59)).toBe('common')
  })

  it('#3 rand=0.60 → uncommon (刚好超过 common 60% 边界)', () => {
    expect(rollRarity(0.60)).toBe('uncommon')
  })

  it('#4 rand=0.95 → epic (95% 落在 epic 4% 区间 0.95~0.99)', () => {
    expect(rollRarity(0.95)).toBe('epic')
  })

  it('#5 rand=0.99 → legendary (99% 落在 legendary 1% 区间 0.96~1.0)', () => {
    expect(rollRarity(0.99)).toBe('legendary')
  })
})

describe('rollSpecies', () => {
  it('#6 rand=0.0 → cat', () => {
    expect(rollSpecies(0.0)).toBe('cat')
  })

  it('#7 rand=0.4 → goose (中间值)', () => {
    expect(rollSpecies(0.4)).toBe('goose')
  })

  it('#8 rand=0.8 → dog', () => {
    expect(rollSpecies(0.8)).toBe('dog')
  })
})

describe('generateRandomPosition', () => {
  it('#9 生成的坐标在 minMeters~maxMeters 距离范围内', () => {
    const pos = generateRandomPosition(NINGBO, 50, 500, 0.5, 0.0)
    const dist = calculateDistance(NINGBO, pos)
    expect(dist).toBeGreaterThanOrEqual(40) // 容差 10m
    expect(dist).toBeLessThanOrEqual(510)
  })

  it('#10 rand=0, angleRand=0 → 正北方向，距离=minMeters', () => {
    const pos = generateRandomPosition(NINGBO, 100, 500, 0.0, 0.0)
    expect(pos.lat).toBeGreaterThan(NINGBO.lat) // 正北 → 纬度增大
    expect(pos.lng).toBeCloseTo(NINGBO.lng, 5) // 经度几乎不变
  })

  it('#11 rand=1 → 距离=maxMeters', () => {
    const pos = generateRandomPosition(NINGBO, 50, 500, 1.0, 0.0)
    const dist = calculateDistance(NINGBO, pos)
    expect(dist).toBeGreaterThan(450) // 接近 500m
  })
})

describe('calculateDistance', () => {
  it('#12 同一点距离为 0', () => {
    expect(calculateDistance(NINGBO, NINGBO)).toBe(0)
  })

  it('#13 向北 100m 的点距离约 100m', () => {
    const north100m = { lat: NINGBO.lat + 100 / 111000, lng: NINGBO.lng }
    expect(calculateDistance(NINGBO, north100m)).toBe(100)
  })

  it('#14 向东 100m 的点距离约 100m（考虑经度修正）', () => {
    const east100m = { lat: NINGBO.lat, lng: NINGBO.lng + 100 / (111000 * Math.cos(NINGBO.lat * Math.PI / 180)) }
    expect(calculateDistance(NINGBO, east100m)).toBe(100)
  })
})

describe('projectToCanvas', () => {
  it('#15 中心点投影到画布中心 (50, 50)', () => {
    const result = projectToCanvas(NINGBO, NINGBO, 500)
    expect(result).not.toBeNull()
    expect(result!.x).toBeCloseTo(50, 1)
    expect(result!.y).toBeCloseTo(50, 1)
  })

  it('#16 超出半径的点返回 null', () => {
    const farPoint = { lat: NINGBO.lat + 0.01, lng: NINGBO.lng } // 约 1.1km 外
    expect(projectToCanvas(farPoint, NINGBO, 500)).toBeNull()
  })
})

describe('shouldRefresh', () => {
  it('#17 距上次刷新超过间隔 → true', () => {
    const now = Date.now()
    const lastRefresh = now - REFRESH_INTERVAL_MS - 1
    expect(shouldRefresh(lastRefresh, now, REFRESH_INTERVAL_MS)).toBe(true)
  })

  it('#18 距上次刷新未超过间隔 → false', () => {
    const now = Date.now()
    const lastRefresh = now - 1000
    expect(shouldRefresh(lastRefresh, now, REFRESH_INTERVAL_MS)).toBe(false)
  })
})

describe('filterExpired', () => {
  it('#19 过滤掉 expiresAt < now 的发现点', () => {
    const now = Date.now()
    const points = [
      { id: '1', spawnedAt: now - 600000, expiresAt: now - 1000, status: 'available' as const, species: 'cat' as const, rarity: 'common' as const, lat: 29.87, lng: 121.55 },
      { id: '2', spawnedAt: now - 1000, expiresAt: now + 600000, status: 'available' as const, species: 'dog' as const, rarity: 'rare' as const, lat: 29.88, lng: 121.56 },
    ]
    const result = filterExpired(points, now)
    expect(result).toHaveLength(1)
    expect(result[0].id).toBe('2')
  })
})

describe('generateDiscoveryPoints', () => {
  it('#20 生成指定数量的发现点，且所有点在 50~500m 范围内', () => {
    const now = Date.now()
    const points = generateDiscoveryPoints(NINGBO, 5, now, () => 0.5)
    expect(points).toHaveLength(5)
    points.forEach(p => {
      const dist = calculateDistance(NINGBO, { lat: p.lat, lng: p.lng })
      expect(dist).toBeGreaterThanOrEqual(40)
      expect(dist).toBeLessThanOrEqual(510)
      expect(p.expiresAt).toBe(now + DISCOVERY_TTL_MS)
      expect(p.spawnedAt).toBe(now)
      expect(p.status).toBe('available')
    })
  })
})

describe('updatePointStatus', () => {
  it('#21 距离 ≤ CAPTURE_RANGE_M → in_range', () => {
    const now = Date.now()
    const points = [{
      id: '1', spawnedAt: now, expiresAt: now + 600000, status: 'available' as const,
      species: 'cat' as const, rarity: 'common' as const,
      lat: NINGBO.lat + 30 / 111000, lng: NINGBO.lng, // 北方 30m
    }]
    const result = updatePointStatus(points, NINGBO)
    expect(result[0].status).toBe('in_range')
  })

  it('#22 距离 > CAPTURE_RANGE_M → available', () => {
    const now = Date.now()
    const points = [{
      id: '1', spawnedAt: now, expiresAt: now + 600000, status: 'available' as const,
      species: 'cat' as const, rarity: 'common' as const,
      lat: NINGBO.lat + 200 / 111000, lng: NINGBO.lng, // 北方 200m
    }]
    const result = updatePointStatus(points, NINGBO)
    expect(result[0].status).toBe('available')
  })
})

describe('createPointId', () => {
  it('#23 连续调用生成不同 ID', () => {
    const id1 = createPointId()
    const id2 = createPointId()
    expect(id1).not.toBe(id2)
    expect(id1).toBeTruthy()
  })
})
