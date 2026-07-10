import { useEffect, useMemo, useState } from 'react'
import type { PokedexFilter } from '../data/types'
import PageTitle from '../components/PageTitle'
import RarityCard from '../components/RarityCard'
import { animals as seedAnimals, filterAnimals } from '../data/animals'
import { AnimalRepository } from '../../../db/repositories/animal-repository'

interface PokedexScreenProps {
  onToast: (message: string) => void
}

const filters: { id: PokedexFilter; label: string }[] = [
  { id: 'all', label: '全部' },
  { id: 'cat', label: '猫' },
  { id: 'goose', label: '鹅' },
  { id: 'dog', label: '狗' },
]

type Entry = (typeof seedAnimals)[0]

export default function PokedexScreen({ onToast }: PokedexScreenProps) {
  const [filter, setFilter] = useState<PokedexFilter>('all')
  const [entries, setEntries] = useState<Entry[]>(seedAnimals)

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        const rows = await AnimalRepository.getAll()
        if (cancelled || !rows?.length) return
        const mapped: Entry[] = rows.map((r, idx) => ({
          id: r.id || `idb-${idx}`,
          name: (r as { species?: string }).species || r.id || 'unknown',
          species: ((r as { species?: string }).species as Entry['species']) || 'goose',
          rarity: ((r as { rarity?: string }).rarity as Entry['rarity']) || 'common',
          collected: Boolean((r as { unlocked?: boolean }).unlocked ?? true),
        }))
        setEntries(mapped)
      } catch {
        // keep seed fallback
      }
    })()
    return () => {
      cancelled = true
    }
  }, [])

  const filtered = useMemo(() => filterAnimals(entries as typeof seedAnimals, filter), [entries, filter])
  const collectedCount = entries.filter((e) => e.collected).length

  const handleCardClick = (entry: Entry) => {
    if (!entry.collected) {
      onToast('尚未发现')
      return
    }
    onToast(`${entry.name} · 已贴进手账`)
  }

  return (
    <div className="ap-screen">
      <PageTitle
        title="图鉴"
        subtitle="POKEDEX · 贴纸收藏册"
        rightText={`已收集 ${collectedCount}`}
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

      <div className="ap-pokedex-grid">
        {filtered.map((entry) => (
          <RarityCard key={entry.id} entry={entry} onClick={() => handleCardClick(entry)} />
        ))}
      </div>
    </div>
  )
}
