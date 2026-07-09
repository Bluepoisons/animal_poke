import { useState } from 'react'
import type { PokedexFilter } from '../data/types'
import PageTitle from '../components/PageTitle'
import RarityCard from '../components/RarityCard'
import { animals, filterAnimals, collectedCount } from '../data/animals'

interface PokedexScreenProps {
  onToast: (message: string) => void
}

const filters: { id: PokedexFilter; label: string }[] = [
  { id: 'all', label: '全部' },
  { id: 'cat', label: '猫' },
  { id: 'goose', label: '鹅' },
  { id: 'dog', label: '狗' },
]

export default function PokedexScreen({ onToast }: PokedexScreenProps) {
  const [filter, setFilter] = useState<PokedexFilter>('all')
  const filtered = filterAnimals(animals, filter)

  const handleCardClick = (entry: (typeof animals)[0]) => {
    if (!entry.collected) {
      onToast('尚未发现')
    }
  }

  return (
    <div className="ap-screen">
      <PageTitle
        title="图鉴 POKEDEX"
        rightText={`已收集 ${collectedCount}`}
        rightTone="purple"
      />
      <nav className="ap-pokedex-tabs" aria-label="图鉴过滤">
        {filters.map((f) => (
          <button
            key={f.id}
            className={filter === f.id ? 'is-active' : ''}
            onClick={() => setFilter(f.id)}
            type="button"
          >
            {f.label}
          </button>
        ))}
      </nav>
      <div className="ap-pokedex-grid">
        {filtered.map((entry) => (
          <RarityCard
            key={entry.id}
            entry={entry}
            onClick={() => handleCardClick(entry)}
          />
        ))}
      </div>
    </div>
  )
}
