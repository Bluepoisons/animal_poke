import React from 'react'
import type { CardEntry } from '../types'
import { RARITY_COLORS, RARITY_NAMES, SPECIES_DEFS, getCardSpecies } from '../types'
import { useStatus } from '../status/useStatus'

interface DetailPopupProps {
  entry: CardEntry
  onClose: () => void
  onViewOnMap: (entry: CardEntry) => void
}

const DetailPopup: React.FC<DetailPopupProps> = ({ entry, onClose, onViewOnMap }) => {
  const color = RARITY_COLORS[entry.rarity]
  const statusCtx = useStatus()
  const statusDisplays = statusCtx.getPetStatusDisplay(entry.id)
  const hasCold = statusCtx.hasStatus(entry.id, 'cold')

  return (
    <div style={styles.overlay} onClick={onClose}>
      <div style={styles.card} onClick={e => e.stopPropagation()}>
        <button style={styles.close} onClick={onClose}>✕</button>

        <div style={{ ...styles.photo, background: gradientCSS(entry.seed) }} />

        <div style={styles.no}>{entry.no}</div>

        <span style={{ ...styles.rarityBadge, background: color }}>
          {RARITY_NAMES[entry.rarity]}
        </span>

        <div style={styles.metaList}>
          <div style={styles.metaRow}>
            <span>{SPECIES_DEFS[getCardSpecies(entry)].emoji}</span>
            <span style={styles.metaLabel}>物种</span>
            <span style={styles.metaValue}>{SPECIES_DEFS[getCardSpecies(entry)].name}</span>
          </div>
          <div style={styles.metaRow}>
            <span>📍</span>
            <span style={styles.metaLabel}>捕捉地点</span>
            <span style={styles.metaValue}>{entry.location}</span>
          </div>
          <div style={styles.metaRow}>
            <span>🗓</span>
            <span style={styles.metaLabel}>捕捉日期</span>
            <span style={styles.metaValue}>2026.{entry.captureDate}</span>
          </div>
          <div style={styles.metaRow}>
            <span>🌐</span>
            <span style={styles.metaLabel}>坐标</span>
            <span style={styles.metaValue}>
              {entry.lat.toFixed(2)}, {entry.lng.toFixed(2)}
            </span>
          </div>
        </div>

        <div style={styles.statusSection}>
          <div style={styles.metaRow}>
            <span>📊</span>
            <span style={styles.metaLabel}>状态</span>
          </div>
          {statusDisplays.map((s, i) => (
            <div key={i} style={{ ...styles.statusBadge, color: s.color }}>
              <span style={{ fontSize: 16 }}>{s.emoji}</span>
              <span style={{ fontWeight: 600 }}>{s.label}</span>
              <span style={{ fontSize: 10, color: 'var(--ink-3)' }}>{s.description}</span>
              {s.remainingDays != null && (
                <span style={{ fontSize: 10, color: 'var(--warn)' }}>
                  剩余 {s.remainingDays} 天
                </span>
              )}
            </div>
          ))}
        </div>

        {hasCold && (
          <button
            className="btn btn-primary"
            style={styles.cureBtn}
            onClick={() => {
              const result = statusCtx.cureCold(entry.id)
              if (!result.success) {
                alert(result.reason === 'no_medicine'
                  ? '没有感冒药！请去商店购买（200 金币）'
                  : '宠物未感冒')
              }
            }}
          >
            💊 使用感冒药治疗
          </button>
        )}

        <button
          className="btn btn-primary"
          style={styles.mapBtn}
          onClick={() => onViewOnMap(entry)}
        >
          🗺️ 在地图查看
        </button>
      </div>
    </div>
  )
}

function gradientCSS(seed: number): string {
  const h1 = 10 + (seed * 40) % 50
  const h2 = 15 + (seed * 47) % 55
  return `linear-gradient(135deg, hsl(${h1},60%,92%), hsl(${h2},50%,85%))`
}

const styles: Record<string, React.CSSProperties> = {
  overlay: {
    position: 'absolute',
    inset: 0,
    background: 'rgba(74,44,26,0.6)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    zIndex: 50,
    padding: 20,
  },
  card: {
    width: '100%',
    maxWidth: 320,
    background: 'var(--white)',
    borderRadius: 20,
    padding: 14,
    position: 'relative',
    boxShadow: '0 8px 32px rgba(230,115,0,0.25)',
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    gap: 8,
  },
  close: {
    position: 'absolute',
    top: 8,
    right: 8,
    width: 28,
    height: 28,
    borderRadius: 14,
    border: 'none',
    background: 'var(--orange-50)',
    color: 'var(--ink-3)',
    fontSize: 14,
    cursor: 'pointer',
    display: 'grid',
    placeItems: 'center',
  },
  photo: {
    width: '100%',
    height: 150,
    borderRadius: 16,
  },
  no: {
    fontSize: 22,
    fontWeight: 700,
    color: 'var(--orange-dark)',
  },
  rarityBadge: {
    color: 'var(--white)',
    fontSize: 11,
    fontWeight: 700,
    padding: '2px 12px',
    borderRadius: 12,
  },
  metaList: {
    width: '100%',
    display: 'flex',
    flexDirection: 'column',
    gap: 6,
  },
  metaRow: {
    display: 'flex',
    alignItems: 'center',
    gap: 8,
    fontSize: 12,
  },
  metaLabel: {
    color: 'var(--ink-3)',
  },
  metaValue: {
    marginLeft: 'auto',
    fontWeight: 600,
    color: 'var(--ink)',
  },
  mapBtn: {
    width: '100%',
    marginTop: 4,
    padding: '8px 0',
    fontSize: 13,
    borderRadius: 14,
  },
  statusSection: {
    width: '100%',
    display: 'flex',
    flexDirection: 'column',
    gap: 4,
  },
  statusBadge: {
    display: 'flex',
    alignItems: 'center',
    gap: 6,
    fontSize: 12,
    background: 'var(--orange-50)',
    borderRadius: 10,
    padding: '4px 10px',
  },
  cureBtn: {
    width: '100%',
    marginTop: 4,
    padding: '8px 0',
    fontSize: 13,
    borderRadius: 14,
  },
}

export default DetailPopup
