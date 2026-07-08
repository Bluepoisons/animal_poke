import React, { useState, useCallback, useEffect } from 'react'
import type { CardEntry } from '../types'
import { RARITY_COLORS, SPECIES_DEFS } from '../types'
import { useLbs } from '../lbs/useLbs'
import { projectToCanvas, calculateDistance } from '../lbs/logic'
import { DISCOVERY_RANGE_M } from '../lbs/constants'
import DiscoveryCard from './DiscoveryCard'
import type { DiscoveryPoint } from '../lbs/types'

interface MapScreenProps {
  entries: CardEntry[]
  focusEntry?: CardEntry
  onBack: () => void
}

const REGIONS = [
  { label: 'Whisker Woods', x: 12, y: 14 },
  { label: 'Purrington Fields', x: 63, y: 28 },
  { label: 'Meowridge Hills', x: 20, y: 58 },
  { label: 'Clawport Harbor', x: 65, y: 78 },
]

const MapScreen: React.FC<MapScreenProps> = ({ entries, focusEntry, onBack }) => {
  const { state, requestLocation, refreshPoints, nextRefreshIn } = useLbs()
  const [selectedIdx, setSelectedIdx] = useState<number>(-1)
  const [selectedDiscovery, setSelectedDiscovery] = useState<DiscoveryPoint | null>(null)

  // 进入地图时请求定位
  useEffect(() => {
    if (state.geoStatus === 'idle') {
      requestLocation()
    }
  }, [state.geoStatus, requestLocation])

  // 已捕获标记位置（LBS 模式用玩家中心投影，降级用全量范围映射）
  const markerPositions = useCalcPositions(entries, state.playerLocation)

  // 发现点投影位置
  const discoveryPositions = useDiscoveryPositions(state.discoveryPoints, state.playerLocation)

  const handleCanvasClick = useCallback((e: React.MouseEvent<HTMLDivElement>) => {
    const rect = e.currentTarget.getBoundingClientRect()
    const cx = ((e.clientX - rect.left) / rect.width) * 100
    const cy = ((e.clientY - rect.top) / rect.height) * 100

    // 检查是否点击了发现点
    let bestDiscovery: DiscoveryPoint | null = null
    let bestDist = Infinity
    discoveryPositions.forEach((pos, i) => {
      if (!pos) return
      const d = Math.hypot(cx - pos.x, cy - pos.y)
      if (d < 8 && d < bestDist) {
        bestDist = d
        bestDiscovery = state.discoveryPoints[i]
      }
    })
    if (bestDiscovery) {
      setSelectedDiscovery(bestDiscovery)
      setSelectedIdx(-1)
      return
    }

    // 检查是否点击了已捕获标记
    let best = -1
    let bestEntryDist = Infinity
    markerPositions.forEach((pos, i) => {
      if (pos.x < 0) return
      const d = Math.hypot(cx - pos.x, cy - pos.y)
      if (d < 10 && d < bestEntryDist) {
        bestEntryDist = d
        best = i
      }
    })
    setSelectedIdx(best)
    setSelectedDiscovery(null)
  }, [markerPositions, discoveryPositions, state.discoveryPoints])

  const selected = selectedIdx >= 0 ? entries[selectedIdx] : null

  // 刷新倒计时格式化
  const refreshCountdown = formatCountdown(nextRefreshIn)

  // 城市名显示
  const cityDisplay = state.cityName || '未知城市'

  // 降级提示
  const showDegradation = state.geoStatus === 'denied' || state.geoStatus === 'unsupported' || state.geoStatus === 'timeout'

  // 玩家位置投影
  const playerPos = state.playerLocation
    ? projectToCanvas(state.playerLocation, state.playerLocation, DISCOVERY_RANGE_M)
    : null

  return (
    <div style={styles.container}>
      {/* Header */}
      <div style={styles.header}>
        <button style={styles.backBtn} onClick={onBack}>‹</button>
        <span style={styles.headerTitle}>🐾 Hunt Map</span>
        <span style={styles.headerCity}>📍 {cityDisplay}</span>
        {state.geoStatus === 'located' && (
          <span style={styles.headerCountdown}>🔄 {refreshCountdown}</span>
        )}
      </div>

      {/* 降级提示条 */}
      {showDegradation && (
        <div style={styles.degradationBanner}>
          <span>⚠️ {state.errorMsg || '无法获取位置'}</span>
          <button style={styles.retryBtn} onClick={requestLocation}>
            {state.geoStatus === 'timeout' ? '重试' : '重新授权'}
          </button>
        </div>
      )}

      {/* Canvas */}
      <div style={styles.canvas} onClick={handleCanvasClick}>
        {/* Forest */}
        <div style={{ ...styles.forest }} />
        {/* River */}
        <div style={styles.river}>
          <div style={styles.riverDash} />
        </div>
        {/* Mountain */}
        <div style={styles.mountain} />

        {/* Region labels */}
        {REGIONS.map((r, i) => (
          <span key={i} style={{ ...styles.regionLabel, left: `${r.x}%`, top: `${r.y}%` }}>
            {r.label}
          </span>
        ))}

        {/* Path line */}
        <svg style={styles.svgOverlay} viewBox="0 0 100 100" preserveAspectRatio="none">
          <polyline
            points="20,24 55,42 38,60 70,76"
            fill="none"
            stroke="var(--orange)"
            strokeWidth="1.2"
            strokeDasharray="3,5"
            opacity="0.55"
          />
        </svg>

        {/* 玩家位置标记 */}
        {playerPos && (
          <div style={{
            position: 'absolute',
            left: `${playerPos.x}%`,
            top: `${playerPos.y}%`,
            transform: 'translate(-50%, -50%)',
            zIndex: 4,
          }}>
            {/* 精度圈 */}
            {state.playerLocation?.accuracy && (
              <div style={{
                position: 'absolute',
                left: '50%',
                top: '50%',
                transform: 'translate(-50%, -50%)',
                width: Math.min(state.playerLocation.accuracy / DISCOVERY_RANGE_M * 90, 30),
                height: Math.min(state.playerLocation.accuracy / DISCOVERY_RANGE_M * 90, 30),
                borderRadius: '50%',
                background: 'rgba(59, 130, 246, 0.15)',
                border: '1px solid rgba(59, 130, 246, 0.3)',
              }} />
            )}
            {/* 玩家蓝点 */}
            <div style={{
              width: 16,
              height: 16,
              borderRadius: '50%',
              background: '#3B82F6',
              border: '3px solid var(--white)',
              boxShadow: '0 2px 6px rgba(59,130,246,0.5)',
              position: 'relative',
              zIndex: 1,
            }} />
          </div>
        )}

        {/* 已捕获 Markers */}
        {markerPositions.map((pos, i) => {
          if (pos.x < 0) return null
          const entry = entries[i]
          if (!entry) return null
          const color = RARITY_COLORS[entry.rarity]
          const isSel = i === selectedIdx

          return (
            <div key={`entry-${i}`} style={{
              position: 'absolute',
              left: `${pos.x}%`,
              top: `${pos.y}%`,
              transform: 'translate(-50%, -100%)',
              zIndex: 3,
              cursor: 'pointer',
            }}>
              {/* Pin body */}
              <div style={{
                width: 28,
                height: 28,
                borderRadius: '50% 50% 50% 0',
                background: color,
                transform: 'rotate(-45deg)',
                boxShadow: `0 3px 0 rgba(230,115,0,0.3)`,
                display: 'grid',
                placeItems: 'center',
              }}>
                <span style={{ transform: 'rotate(45deg)', fontSize: 14 }}>🐾</span>
              </div>
              {/* Photo thumbnail */}
              <div style={{
                position: 'absolute',
                left: '50%',
                top: '50%',
                transform: 'translate(-50%, -50%)',
                width: 18,
                height: 18,
                borderRadius: '50%',
                border: '2px solid var(--white)',
                overflow: 'hidden',
                background: `linear-gradient(135deg, hsl(${25 + entry.seed * 20},55%,92%), hsl(${35 + entry.seed * 23},45%,82%))`,
              }} />
              {/* Selection ring */}
              {isSel && (
                <div style={{
                  position: 'absolute',
                  top: -22,
                  left: -2,
                  width: 32,
                  height: 32,
                  borderRadius: '50%',
                  border: '2px solid var(--white)',
                }} />
              )}
            </div>
          )
        })}

        {/* 发现点 Markers */}
        {discoveryPositions.map((pos, i) => {
          if (!pos) return null
          const point = state.discoveryPoints[i]
          if (!point) return null
          const color = RARITY_COLORS[point.rarity]
          const speciesEmoji = SPECIES_DEFS[point.species].emoji
          const isInRange = point.status === 'in_range'
          const dist = state.playerLocation
            ? calculateDistance({ lat: point.lat, lng: point.lng }, state.playerLocation)
            : null

          return (
            <div key={`discovery-${point.id}`} style={{
              position: 'absolute',
              left: `${pos.x}%`,
              top: `${pos.y}%`,
              transform: 'translate(-50%, -50%)',
              zIndex: isInRange ? 5 : 3,
              cursor: 'pointer',
            }}>
              {/* 闪烁动画 pin */}
              <div style={{
                width: isInRange ? 36 : 24,
                height: isInRange ? 36 : 24,
                borderRadius: '50%',
                background: color,
                border: isInRange ? '3px solid var(--white)' : '2px solid var(--white)',
                boxShadow: `0 2px 8px ${isInRange ? 'rgba(230,115,0,0.5)' : 'rgba(0,0,0,0.2)'}`,
                display: 'grid',
                placeItems: 'center',
                animation: 'pulse 2s infinite',
              }}>
                <span style={{ fontSize: isInRange ? 18 : 14 }}>{speciesEmoji}</span>
              </div>
              {/* in_range 脉冲光圈 */}
              {isInRange && (
                <div style={{
                  position: 'absolute',
                  left: '50%',
                  top: '50%',
                  transform: 'translate(-50%, -50%)',
                  width: 48,
                  height: 48,
                  borderRadius: '50%',
                  border: `2px solid ${color}`,
                  animation: 'pulse-ring 2s infinite',
                }} />
              )}
              {/* 距离标签 */}
              {dist !== null && (
                <span style={{
                  position: 'absolute',
                  top: isInRange ? 40 : 28,
                  left: '50%',
                  transform: 'translateX(-50%)',
                  fontSize: 10,
                  color: 'var(--ink-3)',
                  whiteSpace: 'nowrap',
                }}>
                  {dist}m
                </span>
              )}
            </div>
          )
        })}

        {/* 已捕获 Popup */}
        {selected && (
          <div style={styles.popup}>
            <div style={{
              ...styles.popupPhoto,
              background: `linear-gradient(135deg, hsl(${25 + selected.seed * 20},55%,92%), hsl(${35 + selected.seed * 23},45%,82%))`,
            }} />
            <div style={styles.popupInfo}>
              <div style={styles.popupNo}>{selected.no}</div>
              <div style={styles.popupLoc}>📍 {selected.location}</div>
              <div style={styles.popupDate}>🗓 2026.{selected.captureDate}</div>
            </div>
          </div>
        )}

        {/* 发现点 Popup */}
        {selectedDiscovery && (
          <DiscoveryCard
            point={selectedDiscovery}
            onClose={() => setSelectedDiscovery(null)}
          />
        )}

        {/* 手动刷新按钮 */}
        {state.geoStatus === 'located' && (
          <button style={styles.refreshBtn} onClick={() => refreshPoints()}>
            🔄 刷新
          </button>
        )}
      </div>

      {/* CSS 动画 */}
      <style>{`
        @keyframes pulse {
          0%, 100% { transform: scale(1); opacity: 1; }
          50% { transform: scale(1.1); opacity: 0.8; }
        }
        @keyframes pulse-ring {
          0% { transform: translate(-50%, -50%) scale(0.8); opacity: 1; }
          100% { transform: translate(-50%, -50%) scale(1.5); opacity: 0; }
        }
      `}</style>
    </div>
  )
}

/** 计算已捕获标记位置（支持 LBS 投影 + 降级回退） */
function useCalcPositions(entries: CardEntry[], playerLocation?: { lat: number; lng: number } | null) {
  return React.useMemo(() => {
    // LBS 模式：以玩家位置为中心投影
    if (playerLocation) {
      return entries.map(e => {
        if (e.lat === 0 && e.lng === 0) return { x: -1, y: -1 }
        const result = projectToCanvas({ lat: e.lat, lng: e.lng }, playerLocation, DISCOVERY_RANGE_M)
        if (!result) return { x: -1, y: -1 }
        return { x: result.x, y: result.y }
      })
    }

    // 降级模式：全量范围映射
    const valid = entries.filter(e => e.lat !== 0 || e.lng !== 0)
    if (valid.length === 0) return entries.map(() => ({ x: -1, y: -1 }))

    const lats = valid.map(e => e.lat)
    const lngs = valid.map(e => e.lng)
    const minLat = Math.min(...lats)
    const maxLat = Math.max(...lats)
    const minLng = Math.min(...lngs)
    const maxLng = Math.max(...lngs)
    const pad = 0.18
    const latRange = Math.max(maxLat - minLat, 0.001)
    const lngRange = Math.max(maxLng - minLng, 0.001)

    return entries.map(e => {
      if (e.lat === 0 && e.lng === 0) return { x: -1, y: -1 }
      const nx = pad + (1 - 2 * pad) * (e.lng - minLng) / lngRange
      const ny = 1 - (pad + (1 - 2 * pad) * (e.lat - minLat) / latRange)
      return { x: nx * 100, y: ny * 100 }
    })
  }, [entries, playerLocation])
}

/** 计算发现点投影位置 */
function useDiscoveryPositions(points: DiscoveryPoint[], playerLocation: { lat: number; lng: number } | null) {
  return React.useMemo(() => {
    if (!playerLocation) return points.map(() => null)
    return points.map(p => projectToCanvas({ lat: p.lat, lng: p.lng }, playerLocation, DISCOVERY_RANGE_M))
  }, [points, playerLocation])
}

/** 格式化刷新倒计时（秒 → mm:ss） */
function formatCountdown(seconds: number): string {
  if (seconds <= 0) return '00:00'
  const m = Math.floor(seconds / 60)
  const s = seconds % 60
  return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    position: 'absolute',
    inset: 0,
    background: 'var(--cream)',
    zIndex: 30,
    display: 'flex',
    flexDirection: 'column',
  },
  header: {
    height: 50,
    background: 'var(--orange-dark)',
    borderRadius: '0 0 16px 16px',
    display: 'flex',
    alignItems: 'center',
    gap: 8,
    padding: '0 12px',
    color: 'var(--white)',
    flexShrink: 0,
  },
  backBtn: {
    width: 32,
    height: 32,
    borderRadius: 10,
    border: 'none',
    background: 'rgba(255,255,255,0.2)',
    color: 'var(--white)',
    fontSize: 22,
    display: 'grid',
    placeItems: 'center',
    cursor: 'pointer',
    fontFamily: 'inherit',
    lineHeight: 1,
  },
  headerTitle: {
    fontSize: 16,
    fontWeight: 700,
  },
  headerCity: {
    fontSize: 13,
    fontWeight: 500,
  },
  headerCountdown: {
    fontSize: 12,
    fontWeight: 500,
    marginLeft: 'auto',
  },
  degradationBanner: {
    background: '#FEF3C7',
    color: '#92400E',
    fontSize: 13,
    padding: '8px 12px',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: 8,
    borderRadius: '0 0 8px 8px',
    flexShrink: 0,
  },
  retryBtn: {
    height: 28,
    borderRadius: 8,
    border: '1px solid #92400E',
    background: 'transparent',
    color: '#92400E',
    fontSize: 12,
    cursor: 'pointer',
    fontFamily: 'inherit',
    padding: '0 8px',
  },
  canvas: {
    flex: 1,
    position: 'relative',
    overflow: 'hidden',
    background: 'linear-gradient(180deg, #E6F0E0 0%, #F2E9C9 55%, #D8E8F0 100%)',
    margin: 0,
    borderRadius: 16,
  },
  forest: {
    position: 'absolute',
    width: 100,
    height: 80,
    background: 'radial-gradient(circle at 30% 40%, #7DBB74 0%, transparent 40%), radial-gradient(circle at 70% 60%, #6DAF64 0%, transparent 40%)',
    borderRadius: '40% 60% 50% 50%',
    top: '10%',
    left: '8%',
    opacity: 0.6,
  },
  river: {
    position: 'absolute',
    width: '60%',
    height: 20,
    background: 'linear-gradient(90deg, transparent, #7FB6D9, #A8D8EA, transparent)',
    borderRadius: 12,
    transform: 'rotate(-12deg)',
    top: '42%',
    left: '12%',
    opacity: 0.75,
  },
  riverDash: {
    position: 'absolute',
    inset: 0,
    border: '2px dashed rgba(255,255,255,0.6)',
    borderRadius: 12,
  },
  mountain: {
    position: 'absolute',
    width: 0,
    height: 0,
    borderLeft: '48px solid transparent',
    borderRight: '48px solid transparent',
    borderBottom: '80px solid #C9B89B',
    bottom: '28%',
    left: '56%',
    filter: 'drop-shadow(0 4px 0 rgba(0,0,0,0.05))',
  },
  regionLabel: {
    position: 'absolute',
    fontFamily: "'Comic Sans MS', 'Bradley Hand', cursive",
    fontSize: 12,
    color: 'var(--ink-2)',
    fontWeight: 600,
    textShadow: '0 1px 0 rgba(255,255,255,0.6)',
  },
  svgOverlay: {
    position: 'absolute',
    inset: 0,
    width: '100%',
    height: '100%',
    pointerEvents: 'none' as const,
  },
  popup: {
    position: 'absolute',
    bottom: 16,
    left: '50%',
    transform: 'translateX(-50%)',
    width: 260,
    background: 'var(--white)',
    borderRadius: 18,
    boxShadow: 'var(--shadow-card)',
    padding: 12,
    display: 'flex',
    gap: 10,
    alignItems: 'center',
    zIndex: 5,
  },
  popupPhoto: {
    width: 64,
    height: 64,
    borderRadius: 14,
    flexShrink: 0,
  },
  popupInfo: {
    flex: 1,
    display: 'flex',
    flexDirection: 'column',
    gap: 2,
  },
  popupNo: {
    fontSize: 14,
    fontWeight: 700,
    color: 'var(--orange-dark)',
  },
  popupLoc: {
    fontSize: 12,
    color: 'var(--ink-2)',
  },
  popupDate: {
    fontSize: 11,
    color: 'var(--ink-3)',
  },
  refreshBtn: {
    position: 'absolute',
    top: 8,
    right: 8,
    width: 36,
    height: 36,
    borderRadius: '50%',
    border: 'none',
    background: 'rgba(255,255,255,0.8)',
    color: 'var(--orange-dark)',
    fontSize: 16,
    display: 'grid',
    placeItems: 'center',
    cursor: 'pointer',
    boxShadow: '0 2px 6px rgba(0,0,0,0.1)',
    zIndex: 4,
  },
}

export default MapScreen
