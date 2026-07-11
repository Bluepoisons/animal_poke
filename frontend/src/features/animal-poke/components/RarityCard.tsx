import type { AnimalEntry } from '../data/types'
import AnimalIcon from './AnimalIcon'
import { rarityNames } from '../data/animals'
import { useI18n } from '../../../i18n'

interface RarityCardProps {
  entry: AnimalEntry
  onClick?: () => void
}

export default function RarityCard({ entry, onClick }: RarityCardProps) {
  const { t } = useI18n()
  if (!entry.collected) {
    return (
      <article className="ap-rarity-card locked" aria-label={t('rarity.locked')}>
        ???
      </article>
    )
  }

  return (
    <article
      className={`ap-rarity-card ${entry.rarity}`}
      onClick={onClick}
      onKeyDown={(event) => {
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault()
          onClick?.()
        }
      }}
      role="button"
      tabIndex={0}
      aria-label={`${entry.name} ${rarityNames[entry.rarity]}`}
    >
      <div className="ap-rarity-card__icon">
        <AnimalIcon
          species={entry.species}
          size={78}
          tone={entry.rarity === 'legendary' ? 'dark' : 'light'}
        />
      </div>
      <h2>
        #{entry.id} · {rarityNames[entry.rarity]}
      </h2>
      {entry.region && entry.location ? (
        <p>
          {entry.region} · {entry.location}
        </p>
      ) : null}
    </article>
  )
}
