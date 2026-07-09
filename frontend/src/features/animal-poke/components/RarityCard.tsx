import type { AnimalEntry } from '../data/types'
import AnimalIcon from './AnimalIcon'
import { rarityNames } from '../data/animals'

interface RarityCardProps {
  entry: AnimalEntry
  onClick?: () => void
}

export default function RarityCard({ entry, onClick }: RarityCardProps) {
  if (!entry.collected) {
    return (
      <article className="ap-rarity-card locked" aria-label="未解锁">
        ???
      </article>
    )
  }

  return (
    <article
      className={`ap-rarity-card ${entry.rarity}`}
      onClick={onClick}
      role="button"
      tabIndex={0}
      aria-label={`${entry.name} ${rarityNames[entry.rarity]}`}
    >
      <div className="ap-rarity-card__icon">
        <AnimalIcon species={entry.species} size={82} tone={entry.rarity === 'legendary' ? 'dark' : 'light'} />
      </div>
      <h2>
        #{entry.id} · {rarityNames[entry.rarity]}
      </h2>
      {entry.region && entry.location && <p>{entry.region} · {entry.location}</p>}
    </article>
  )
}
