import React, { useState, useCallback } from 'react'
import type { CardEntry } from '../types'
import { RARITY_COLORS } from '../types'

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
  const [selectedIdx, setSelectedIdx] = useState<number>(-1)

  const markerPositions = useCalcPositions(entries)

  const handleCanvasClick = useCallback((e: React.MouseEvent<HTMLDivElement>) => {
    const rect = e.currentTarget.getBoundingClientRect()
    const cx = ((e.clientX - rect.left) / rect.width) * 100
    const cy = ((e.clientY - rect.top) / rect.height) * 100

    let best = -1
    let bestDist = Infinity
    markerPositions.forEach((pos, i) => {
      if (pos.x < 0) return
      const d = Math.hypot(cx - pos.x, cy - pos.y)
      if (d < 10 && d < bestDist) {
        bestDist = d
        best = i
      }
    })
    setSelectedIdx(best)
  }, [markerPositions])

  const selected = selectedIdx >= 0 ? entries[selectedIdx] : null

  return (
    <div style={styles.container}>
      {/* Header */}
      <div style={styles.header}>
        <button style={styles.backBtn} onClick={onBack}>‹</button>
        <span style={styles.headerTitle}>🐾 Hunt Map</span>
      </div>

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

        {/* Markers */}
        {markerPositions.map((pos, i) => {
          if (pos.x < 0) return null
          const entry = entries[i]
          if (!entry) return null
          const color = RARITY_COLORS[entry.rarity]
          const isSel = i === selectedIdx

          return (
            <div key={i} style={{
              position: 'absolute',
              left: `${pos.x}%`,
              top: `${pos.y}%`,
              transform: 'translate(-50%, -100%)',
              zIndex: 3,
              cursor: 'pointer',
            }}>
              {/* Pin body (teardrop) */}
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

        {/* Popup */}
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
      </div>
    </div>
  )
}

function useCalcPositions(entries: CardEntry[]) {
  return React.useMemo(() => {
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
  }, [entries])
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
    gap: 10,
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
}

export default MapScreen
