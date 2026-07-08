import React, { useState, useMemo } from 'react'
import type { CardEntry, FilterTab, RarityTier, SpeciesType } from '../types'
import { RARITY_COLORS, SPECIES_DEFS, getCardSpecies } from '../types'
import { useAnimalStore } from '../hooks/useAnimalStore'
import DetailPopup from './DetailPopup'
import LoadingScreen from './LoadingScreen'

/** 物种筛选类型 */
type SpeciesFilter = 'all' | SpeciesType

interface CollectScreenProps {
  onMapOpen: (entries: CardEntry[], focus?: CardEntry) => void
}

const CollectScreen: React.FC<CollectScreenProps> = ({ onMapOpen }) => {
  const { animals, loading } = useAnimalStore()
  const [filter, setFilter] = useState<FilterTab>('all')
  const [speciesFilter, setSpeciesFilter] = useState<SpeciesFilter>('all')
  const [selectedEntry, setSelectedEntry] = useState<CardEntry | null>(null)

  const filtered = useMemo(() => {
    return animals.filter(e => {
      if (!e.unlocked && filter !== 'all') return false
      switch (filter) {
        case 'today': return e.captureDate === '2026-07-08'
        case 'week': return e.captureDate >= '2026-07-01' && e.captureDate <= '2026-07-08'
        case 'nearby': return e.location.includes('海曙区')
        default: return true
      }
    }).filter(e => {
      // 物种筛选
      if (speciesFilter !== 'all' && getCardSpecies(e) !== speciesFilter) return false
      return true
    })
  }, [filter, speciesFilter, animals])

  // 统计各物种已捕获数量
  const speciesCounts = useMemo(() => {
    const counts: Record<string, number> = { cat: 0, goose: 0, dog: 0 }
    animals.filter(e => e.unlocked).forEach(e => {
      const s = getCardSpecies(e)
      if (counts[s] !== undefined) counts[s]++
    })
    return counts
  }, [animals])

  // Pad to 9 cards (fill with locked placeholders)
  // 为 pad 卡片分配物种以支持物种筛选
  const shown = useMemo(() => {
    const speciesPool: SpeciesType[] = ['cat', 'goose', 'dog']
    const result: CardEntry[] = [...filtered]
    while (result.length < 9) {
      const padSpecies = speciesPool[result.length % 3]
      result.push({
        id: `pad${result.length}`,
        no: '#???',
        rarity: 'common',
        species: padSpecies,
        unlocked: false,
        captureDate: '—',
        location: '待发现',
        lat: 0,
        lng: 0,
        seed: 99,
      })
    }
    return result
  }, [filtered])

  const unlockedCount = animals.filter(e => e.unlocked).length
  const tabs: { key: FilterTab; label: string }[] = [
    { key: 'all', label: '全部' },
    { key: 'today', label: '今日' },
    { key: 'week', label: '本周' },
    { key: 'nearby', label: '附近' },
  ]

  // 物种筛选 tab
  const speciesTabs: { key: SpeciesFilter; label: string; emoji: string }[] = [
    { key: 'all', label: '全部', emoji: '📖' },
    { key: 'cat', label: '猫', emoji: '🐱' },
    { key: 'goose', label: '鹅', emoji: '🪿' },
    { key: 'dog', label: '狗', emoji: '🐶' },
  ]

  return (
    <div style={styles.container}>
      {loading ? (
        <LoadingScreen />
      ) : (
      <>
      {/* Header */}
      <div style={styles.header}>
        <div style={styles.headerContent}>
          {/* Title row */}
          <div style={styles.titleRow}>
            <span style={styles.title}>📖 Collection</span>
            <span style={styles.countPill}>{unlockedCount} / 50</span>
          </div>

          {/* 物种计数 */}
          <div style={styles.speciesCounts}>
            {speciesTabs.filter(t => t.key !== 'all').map(t => (
              <span key={t.key} style={styles.speciesCount}>
                {t.emoji} {speciesCounts[t.key] || 0}
              </span>
            ))}
          </div>

          {/* Search + Map */}
          <div style={styles.actionRow}>
            <input className="input" placeholder="🔍 搜索地点 / 日期…" style={styles.search} />
            <button className="btn btn-primary" style={styles.mapBtn} onClick={() => onMapOpen(filtered)}>
              🗺️
            </button>
          </div>

          {/* Filter tabs */}
          <div style={styles.filterTabs}>
            {tabs.map(t => (
              <button
                key={t.key}
                style={{
                  ...styles.filterTab,
                  ...(filter === t.key ? styles.filterTabActive : {}),
                }}
                onClick={() => setFilter(t.key)}
              >
                {t.label}
              </button>
            ))}
          </div>

          {/* 物种筛选 tab */}
          <div style={styles.filterTabs}>
            {speciesTabs.map(t => (
              <button
                key={t.key}
                style={{
                  ...styles.filterTab,
                  ...(speciesFilter === t.key ? styles.filterTabActive : {}),
                }}
                onClick={() => setSpeciesFilter(t.key)}
              >
                {t.emoji} {t.label}
              </button>
            ))}
          </div>
        </div>
      </div>

      {/* Photo grid */}
      <div style={styles.gridWrap}>
        <div style={styles.grid}>
          {shown.map((entry, idx) => (
            <div
              key={idx}
              style={{
                ...styles.card,
                borderColor: RARITY_COLORS[entry.rarity as RarityTier] || 'var(--rarity-common)',
                cursor: entry.unlocked ? 'pointer' : 'default',
              }}
              onClick={() => entry.unlocked && setSelectedEntry(entry)}
            >
              {entry.unlocked ? (
                <>
                  <div style={{ ...styles.cardPhoto, background: cardGradient(entry.seed) }}>
                    <span style={{ fontSize: 28 }}>{SPECIES_DEFS[getCardSpecies(entry)].emoji}</span>
                    {entry.isNew && <span style={styles.newBadge}>NEW</span>}
                  </div>
                  <div style={styles.cardMeta}>
                    <div style={styles.cardNo}>{entry.no}</div>
                    <div style={styles.cardLoc}>📍 {entry.location}</div>
                    <div style={styles.cardDate}>{entry.captureDate}</div>
                  </div>
                </>
              ) : (
                <>
                  <div style={styles.cardPhotoLocked}>
                    <span style={{ fontSize: 22, color: 'var(--ink-3)' }}>🔒</span>
                  </div>
                  <div style={styles.cardMeta}>
                    <div style={{ ...styles.cardNo, color: 'var(--ink-3)' }}>#???</div>
                    <div style={{ ...styles.cardLoc, color: 'var(--ink-3)' }}>待发现</div>
                    <div style={{ ...styles.cardDate, color: 'var(--ink-3)' }}>—</div>
                  </div>
                </>
              )}
            </div>
          ))}
        </div>
      </div>

      {/* Detail popup */}
      {selectedEntry && (
        <DetailPopup
          entry={selectedEntry}
          onClose={() => setSelectedEntry(null)}
          onViewOnMap={(e) => { setSelectedEntry(null); onMapOpen(filtered, e) }}
        />
      )}
      </>
      )}
    </div>
  )
}

function cardGradient(seed: number): string {
  const h1 = 25 + seed * 20
  const h2 = 35 + seed * 23
  return `linear-gradient(135deg, hsl(${h1 % 360},55%,92%), hsl(${h2 % 360},45%,82%))`
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    flex: 1,
    display: 'flex',
    flexDirection: 'column',
    overflow: 'hidden',
  },
  header: {
    background: 'var(--cream)',
    borderRadius: '0 0 16px 16px',
    flexShrink: 0,
  },
  headerContent: {
    padding: '10px 12px',
    display: 'flex',
    flexDirection: 'column',
    gap: 8,
  },
  titleRow: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
  },
  title: {
    fontSize: 18,
    fontWeight: 700,
    color: 'var(--orange-dark)',
  },
  countPill: {
    background: 'var(--orange-50)',
    color: 'var(--ink-2)',
    border: '2px solid var(--orange-100)',
    borderRadius: 14,
    padding: '2px 10px',
    fontSize: 11,
    fontWeight: 600,
  },
  // 物种计数行
  speciesCounts: {
    display: 'flex',
    gap: 10,
    justifyContent: 'center',
  },
  speciesCount: {
    fontSize: 11,
    fontWeight: 600,
    color: 'var(--ink-2)',
  },
  actionRow: {
    display: 'flex',
    gap: 8,
  },
  search: {
    flex: 1,
    height: 34,
    borderRadius: 12,
  },
  mapBtn: {
    width: 38,
    height: 34,
    fontSize: 16,
    borderRadius: 12,
    boxShadow: '0 4px 0 var(--orange-dark)',
  },
  filterTabs: {
    display: 'flex',
    gap: 6,
  },
  filterTab: {
    flex: 1,
    padding: '4px 0',
    borderRadius: 14,
    fontSize: 11,
    fontWeight: 600,
    border: '2px solid var(--orange-100)',
    background: 'var(--white)',
    color: 'var(--ink-3)',
    cursor: 'pointer',
    fontFamily: 'inherit',
    textAlign: 'center' as const,
  },
  filterTabActive: {
    background: 'var(--orange)',
    color: 'var(--white)',
    borderColor: 'var(--orange)',
  },
  gridWrap: {
    flex: 1,
    overflowY: 'auto' as const,
    padding: '10px 12px',
  },
  grid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(3, 1fr)',
    gap: 10,
  },
  card: {
    aspectRatio: '1 / 1.22',
    background: 'var(--white)',
    borderRadius: 16,
    boxShadow: 'var(--shadow-card)',
    border: '3px solid var(--rarity-common)',
    overflow: 'hidden',
    display: 'flex',
    flexDirection: 'column',
    position: 'relative' as const,
  },
  cardPhoto: {
    flex: 1,
    display: 'grid',
    placeItems: 'center',
    position: 'relative' as const,
  },
  cardPhotoLocked: {
    flex: 1,
    background: '#E8E8E8',
    display: 'grid',
    placeItems: 'center',
  },
  newBadge: {
    position: 'absolute',
    top: 5,
    left: 5,
    background: 'var(--orange)',
    color: 'var(--white)',
    fontSize: 8,
    fontWeight: 700,
    padding: '1px 5px',
    borderRadius: 4,
    letterSpacing: 0.5,
  },
  cardMeta: {
    padding: '5px 7px 7px',
    display: 'flex',
    flexDirection: 'column',
    gap: 1,
  },
  cardNo: {
    fontSize: 11,
    fontWeight: 700,
    color: 'var(--orange-dark)',
  },
  cardLoc: {
    fontSize: 9,
    color: 'var(--ink-2)',
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  },
  cardDate: {
    fontSize: 9,
    color: 'var(--ink-3)',
  },
}

export default CollectScreen
