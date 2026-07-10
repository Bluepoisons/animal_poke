import { useEffect, useMemo, useState } from 'react'
import type { PokedexFilter, AnimalEntry, Rarity, Species } from '../data/types'
import PageTitle from '../components/PageTitle'
import RarityCard from '../components/RarityCard'
import { filterAnimals } from '../data/animals'
import { AnimalRepository } from '../../../db/repositories/animal-repository'
import type { AnimalRecord } from '../../../db/types'
import type { RarityTier, SpeciesType } from '../../../types'

interface PokedexScreenProps {
  onToast: (message: string) => void
}

const filters: { id: PokedexFilter; label: string }[] = [
  { id: 'all', label: '全部' },
  { id: 'cat', label: '猫' },
  { id: 'goose', label: '鹅' },
  { id: 'dog', label: '狗' },
]

function mapRecord(r: AnimalRecord): AnimalEntry {
  const rarity = (r.rarity || 'common') as Rarity
  const species = (r.species || 'cat') as Species
  return {
    id: r.id,
    name: r.species ? String(r.species) : r.no || r.id,
    species,
    rarity,
    collected: Boolean(r.unlocked),
    region: r.location,
    location: r.location,
    captureRate: undefined,
  }
}

export default function PokedexScreen({ onToast }: PokedexScreenProps) {
  const [filter, setFilter] = useState<PokedexFilter>('all')
  const [entries, setEntries] = useState<AnimalEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [selected, setSelected] = useState<AnimalEntry | null>(null)

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        const rows = await AnimalRepository.getAll()
        if (cancelled) return
        setEntries(rows.map(mapRecord))
      } catch {
        if (!cancelled) setEntries([])
      } finally {
        if (!cancelled) setLoading(false)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [])

  const filtered = useMemo(
    () => filterAnimals(entries as Parameters<typeof filterAnimals>[0], filter),
    [entries, filter],
  )
  const collectedCount = entries.filter((e) => e.collected).length

  const handleCardClick = (entry: AnimalEntry) => {
    if (!entry.collected) {
      onToast('尚未发现')
      return
    }
    setSelected(entry)
  }

  return (
    <div className="ap-screen">
      <PageTitle
        title="图鉴"
        subtitle="POKEDEX · 贴纸收藏册"
        rightText={loading ? '加载中…' : `已收集 ${collectedCount}`}
        rightTone="pink"
      />

      <nav className="ap-pokedex-tabs" aria-label="图鉴过滤">
        {filters.map((item) => (
          <button
            key={item.id}
            className={filter === item.id ? 'is-active' : ''}
            onClick={() => setFilter(item.id)}
            type="button"
          >
            {item.label}
          </button>
        ))}
      </nav>

      {!loading && entries.length === 0 && (
        <div role="status" style={{ padding: 24, textAlign: 'center', opacity: 0.8 }}>
          <p style={{ fontWeight: 700 }}>还没有贴纸</p>
          <p style={{ fontSize: 13 }}>去发现页识别并捕获后，这里会自动出现真实收藏。</p>
        </div>
      )}

      <div className="ap-pokedex-grid">
        {filtered.map((entry) => (
          <RarityCard key={entry.id} entry={entry} onClick={() => handleCardClick(entry)} />
        ))}
      </div>

      {selected && (
        <div
          role="dialog"
          aria-modal="true"
          aria-label="图鉴详情"
          style={{
            position: 'fixed',
            inset: 0,
            background: 'rgba(74,44,26,0.35)',
            display: 'grid',
            placeItems: 'center',
            zIndex: 50,
            padding: 16,
          }}
          onClick={() => setSelected(null)}
        >
          <div
            style={{
              background: '#FFFDF8',
              borderRadius: 16,
              padding: 16,
              maxWidth: 320,
              width: '100%',
              border: '2px solid #2B2B2B',
            }}
            onClick={(e) => e.stopPropagation()}
          >
            <h2 style={{ margin: '0 0 8px' }}>{selected.name}</h2>
            <p style={{ margin: 0, fontSize: 13 }}>
              {selected.species} · {selected.rarity}
              {selected.region ? ` · ${selected.region}` : ''}
            </p>
            <button
              type="button"
              className="ap-map-chip"
              style={{ marginTop: 12 }}
              onClick={() => setSelected(null)}
            >
              关闭
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
