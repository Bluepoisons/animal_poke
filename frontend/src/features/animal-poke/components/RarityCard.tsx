import type { AnimalEntry } from '../data/types'
import AnimalIcon from './AnimalIcon'
import { rarityNames } from '../data/animals'
import { useI18n } from '../../../i18n'

interface RarityCardProps {
  entry: AnimalEntry
  nickname?: string
  subtitle?: string
  photoDataUrl?: string
  stats?: { hp?: number; atk?: number }
  onClick?: () => void
}

export default function RarityCard({ entry, nickname, subtitle, photoDataUrl, stats, onClick }: RarityCardProps) {
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
      aria-label={`${nickname || entry.name} ${rarityNames[entry.rarity]}`}
    >
      <div className="ap-rarity-card__icon">
        {photoDataUrl ? (
          <img className="ap-rarity-card__photo" src={photoDataUrl} alt={`${nickname || entry.name} 的照片`} />
        ) : (
          <AnimalIcon
            species={entry.species}
            size={78}
            tone={entry.rarity === 'legendary' ? 'dark' : 'light'}
          />
        )}
      </div>
      <h2>
        {nickname || entry.name} · {rarityNames[entry.rarity]}
      </h2>
      {subtitle && <p>{subtitle}</p>}
      {stats && <div className="ap-rarity-card__stats">生命 {stats.hp ?? '-'} · 攻击 {stats.atk ?? '-'}</div>}
    </article>
  )
}
