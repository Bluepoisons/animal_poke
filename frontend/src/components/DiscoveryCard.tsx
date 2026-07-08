import React from 'react'
import type { DiscoveryPoint } from '../lbs/types'
import { RARITY_COLORS, RARITY_NAMES, SPECIES_DEFS } from '../types'
import { calculateDistance } from '../lbs/logic'
import { useLbs } from '../lbs/useLbs'

interface DiscoveryCardProps {
  point: DiscoveryPoint
  onClose: () => void
  onGoCapture?: (point: DiscoveryPoint) => void
}

/** 发现点信息浮卡：物种 emoji + 稀有度名称 + 距离 + [前往捕获] 按钮 */
const DiscoveryCard: React.FC<DiscoveryCardProps> = ({ point, onClose, onGoCapture }) => {
  const { state } = useLbs()
  const speciesDef = SPECIES_DEFS[point.species]
  const rarityColor = RARITY_COLORS[point.rarity]
  const rarityName = RARITY_NAMES[point.rarity]

  // 计算距离
  const distance = state.playerLocation
    ? calculateDistance({ lat: point.lat, lng: point.lng }, state.playerLocation)
    : null

  const isInRange = point.status === 'in_range'

  return (
    <div style={styles.card}>
      {/* 物种图标 */}
      <div style={{
        ...styles.icon,
        background: rarityColor,
      }}>
        <span style={{ fontSize: 24 }}>{speciesDef.emoji}</span>
      </div>

      {/* 信息区 */}
      <div style={styles.info}>
        <div style={styles.nameRow}>
          <span style={{ fontWeight: 700, fontSize: 16, color: 'var(--orange-dark)' }}>
            {speciesDef.name}
          </span>
          <span style={{
            ...styles.rarityBadge,
            background: rarityColor,
          }}>
            {rarityName}
          </span>
        </div>
        {distance !== null && (
          <div style={styles.distance}>
            📍 距你 {distance}m
          </div>
        )}
        {isInRange && (
          <div style={styles.inRangeTag}>🎯 可捕获</div>
        )}
      </div>

      {/* 操作按钮 */}
      <div style={styles.actions}>
        {isInRange && onGoCapture && (
          <button
            style={styles.captureBtn}
            onClick={() => onGoCapture(point)}
          >
            前往捕获
          </button>
        )}
        <button style={styles.closeBtn} onClick={onClose}>关闭</button>
      </div>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  card: {
    position: 'absolute',
    bottom: 16,
    left: '50%',
    transform: 'translateX(-50%)',
    width: 280,
    background: 'var(--white)',
    borderRadius: 18,
    boxShadow: 'var(--shadow-card)',
    padding: 14,
    display: 'flex',
    flexDirection: 'column',
    gap: 10,
    zIndex: 6,
  },
  icon: {
    width: 56,
    height: 56,
    borderRadius: 14,
    display: 'grid',
    placeItems: 'center',
    alignSelf: 'center',
  },
  info: {
    flex: 1,
    display: 'flex',
    flexDirection: 'column',
    gap: 4,
    textAlign: 'center',
  },
  nameRow: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    gap: 8,
  },
  rarityBadge: {
    fontSize: 11,
    padding: '2px 8px',
    borderRadius: 8,
    color: 'var(--white)',
    fontWeight: 600,
  },
  distance: {
    fontSize: 12,
    color: 'var(--ink-2)',
    textAlign: 'center',
  },
  inRangeTag: {
    fontSize: 13,
    fontWeight: 700,
    color: 'var(--orange-dark)',
    textAlign: 'center',
    padding: '2px 0',
  },
  actions: {
    display: 'flex',
    gap: 8,
    justifyContent: 'center',
  },
  captureBtn: {
    flex: 1,
    height: 36,
    borderRadius: 12,
    border: 'none',
    background: 'var(--orange)',
    color: 'var(--white)',
    fontSize: 14,
    fontWeight: 700,
    cursor: 'pointer',
    fontFamily: 'inherit',
  },
  closeBtn: {
    flex: 1,
    height: 36,
    borderRadius: 12,
    border: '2px solid var(--ink-3)',
    background: 'transparent',
    color: 'var(--ink-2)',
    fontSize: 14,
    cursor: 'pointer',
    fontFamily: 'inherit',
  },
}

export default DiscoveryCard
