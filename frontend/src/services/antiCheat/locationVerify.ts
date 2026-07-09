import type { LocationProof } from './types'

interface GeoLocation {
  lat: number
  lng: number
  accuracy: number
  timestamp: number
}

/** Haversine 距离公式（米） */
function haversineDistance(a: GeoLocation, b: GeoLocation): number {
  const R = 6371000
  const dLat = ((b.lat - a.lat) * Math.PI) / 180
  const dLng = ((b.lng - a.lng) * Math.PI) / 180
  const lat1 = (a.lat * Math.PI) / 180
  const lat2 = (b.lat * Math.PI) / 180
  const h = Math.sin(dLat / 2) ** 2 + Math.cos(lat1) * Math.cos(lat2) * Math.sin(dLng / 2) ** 2
  return 2 * R * Math.asin(Math.sqrt(h))
}

/**
 * 分析位置数据，检测 Mock GPS / 位置欺骗。
 * @param current 当前定位
 * @param previous 上一次定位（可选，用于跳跃检测）
 */
export function analyzeLocation(
  current: GeoLocation,
  previous?: GeoLocation,
): LocationProof {
  const signals: string[] = []
  const now = Date.now()

  // 1. 精度过高（mock GPS 常见特征）
  if (current.accuracy < 3) {
    signals.push(`accuracy_too_high: ${current.accuracy}m`)
  }

  // 2. 位置跳跃检测
  if (previous) {
    const distance = haversineDistance(current, previous)
    const timeDiff = Math.max(1, (current.timestamp - previous.timestamp) / 1000)
    const speed = distance / timeDiff
    // 步行 ~1.4 m/s，高铁 ~100 m/s，阈值 150 m/s
    if (speed > 150) {
      signals.push(`speed_anomaly: ${speed.toFixed(1)}m/s`)
    }
  }

  // 3. 坐标合理性（0,0 或明显测试坐标）
  if (current.lat === 0 && current.lng === 0) {
    signals.push('null_island: lat=0,lng=0')
  }

  return {
    lat: current.lat,
    lng: current.lng,
    accuracy: current.accuracy,
    timestamp: current.timestamp,
    clientTimestamp: now,
    mockDetected: signals.length > 0,
    mockSignals: signals,
  }
}
