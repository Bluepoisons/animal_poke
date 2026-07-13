import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { PokedexFilter, AnimalEntry, Rarity, Species } from '../data/types'
import PageTitle from '../components/PageTitle'
import RarityCard from '../components/RarityCard'
import AnimalIcon from '../components/AnimalIcon'
import { filterAnimals } from '../data/animals'
import { AnimalRepository } from '../../../db/repositories/animal-repository'
import { rarityValueToTier } from '../../../db/animal-record-mapper'
import type { AnimalRecord } from '../../../db/types'
import AccessibleDialog from '../../../a11y/AccessibleDialog'
import { useI18n } from '../../../i18n'
import { LoadingState, EmptyState, ErrorState } from '../../../components/states'
import {
  chinesePetDescription,
  chinesePetSubtitle,
  chineseRarityName,
  chineseDetectedSpeciesName,
  chineseSpeciesGroupName,
  displayPetName,
} from '../petLocalization'
import { speciesGroupOf, type SpeciesGroup } from '../../../species'
import { getCardSpecies } from '../../../types'

interface PokedexScreenProps {
  onToast: (message: string) => void
}

type PokedexEntry = AnimalEntry & {
  record: AnimalRecord
  nickname: string
  subtitle: string
}

const GROUP_ORDER: SpeciesGroup[] = [
  'companion',
  'farm',
  'wildlife',
  'bird',
  'reptile',
  'amphibian',
  'aquatic',
  'insect',
  'other',
]

function mapRecord(r: AnimalRecord): PokedexEntry {
  const rarity = rarityValueToTier(r.rarity) as Rarity
  const species = getCardSpecies(r) as Species
  const name = chineseDetectedSpeciesName(species, r.speciesLabelZh)
  return {
    id: r.id,
    name,
    species,
    rarity,
    collected: Boolean(r.unlocked || r.isUnlocked === 1),
    region: r.location,
    location: r.location,
    captureRate: undefined,
    record: r,
    nickname: displayPetName(r),
    subtitle: chinesePetSubtitle(r),
  }
}

// AP-054: useVirtualList / pickThumbnailSrc available for large collections
export default function PokedexScreen({ onToast }: PokedexScreenProps) {
  const [filter, setFilter] = useState<PokedexFilter>('all')
  const [entries, setEntries] = useState<PokedexEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState(false)
  const [selected, setSelected] = useState<PokedexEntry | null>(null)
  const loadGeneration = useRef(0)
  const { t } = useI18n()

  const loadEntries = useCallback(async () => {
    const generation = ++loadGeneration.current
    setLoading(true)
    setLoadError(false)
    try {
      const rows = await AnimalRepository.getAll()
      if (generation !== loadGeneration.current) return
      setEntries(rows.map(mapRecord))
    } catch {
      if (generation !== loadGeneration.current) return
      setLoadError(true)
    } finally {
      if (generation === loadGeneration.current) setLoading(false)
    }
  }, [])

  useEffect(() => {
    void loadEntries()
    return () => {
      loadGeneration.current += 1
    }
  }, [loadEntries])

  const filtered = useMemo(
    () => filterAnimals(entries, filter) as PokedexEntry[],
    [entries, filter],
  )
  const filters = useMemo<PokedexFilter[]>(() => {
    const present = new Set(entries.map((entry) => speciesGroupOf(entry.species)))
    return ['all', ...GROUP_ORDER.filter((group) => present.has(group))]
  }, [entries])
  const collectedCount = entries.filter((e) => e.collected).length

  const handleCardClick = (entry: PokedexEntry) => {
    if (!entry.collected) {
      onToast(t('pokedex.none'))
      return
    }
    setSelected(entry)
  }

  return (
    <div className="ap-screen" data-testid="pokedex-screen">
      <PageTitle
        title={t('collection.title')}
        subtitle={t('pokedex.subtitle')}
        rightText={loading ? t('common.loading') : t('pokedex.collected', { count: collectedCount })}
        rightTone="pink"
      />

      {loading && <LoadingState title={t('state.loading')} />}

      {loadError && !loading && (
        <ErrorState
          title={t('state.error')}
          body={t('state.error_body')}
          primary={{ label: t('state.error_retry'), onClick: () => void loadEntries() }}
        />
      )}

      {!loading && !loadError && entries.length === 0 && (
        <EmptyState title={t('state.empty')} body={t('state.empty_body')} />
      )}

      {!loading && !loadError && (
        <>
          <nav className="ap-pokedex-tabs" aria-label={t('pokedex.filter_label')}>
            {filters.map((item) => (
              <button
                key={item}
                className={filter === item ? 'is-active' : ''}
                onClick={() => setFilter(item)}
                type="button"
              >
                {item === 'all' ? t('collection.filter.all') : chineseSpeciesGroupName(item)}
              </button>
            ))}
          </nav>

          <div className="ap-pokedex-grid">
            {filtered.map((entry) => (
              <RarityCard
                key={entry.id}
                entry={entry}
                nickname={entry.nickname}
                subtitle={entry.subtitle}
                photoDataUrl={entry.record.photoDataUrl}
                stats={entry.record}
                onClick={() => handleCardClick(entry)}
              />
            ))}
          </div>

          {selected && (
            <AccessibleDialog
              open={!!selected}
              onClose={() => setSelected(null)}
              title={t('pokedex.detail_label')}
            >
              <div className="ap-pet-profile">
                <div className="ap-pet-profile__avatar">
                  {selected.record.photoDataUrl ? (
                    <img src={selected.record.photoDataUrl} alt={`${selected.nickname} 的照片`} />
                  ) : (
                    <AnimalIcon species={selected.species} size={76} tone="light" />
                  )}
                </div>
                <div>
                  <h2 style={{ margin: '0 0 4px' }}>{selected.nickname}</h2>
                  <p className="ap-pet-profile__meta">{chineseDetectedSpeciesName(selected.species, selected.record.speciesLabelZh)} · {chineseRarityName(selected.rarity)}</p>
                  <p className="ap-pet-profile__subtitle">{selected.subtitle}</p>
                </div>
              </div>
              <div className="ap-pet-profile__stats" aria-label="宠物属性">
                {[
                  ['生命', selected.record.hp],
                  ['攻击', selected.record.atk],
                  ['防御', selected.record.def],
                  ['速度', selected.record.spd],
                ].map(([label, value]) => (
                  <div key={label} className="ap-pet-profile__stat"><b>{label}</b><span>{value ?? '-'}</span></div>
                ))}
              </div>
              <p className="ap-pet-profile__narrative">
                {chinesePetDescription(selected.record, selected.nickname)}
              </p>
              <button
                type="button"
                className="ap-map-chip"
                style={{ marginTop: 12 }}
                onClick={() => setSelected(null)}
              >
                {t('pokedex.close_detail')}
              </button>
            </AccessibleDialog>
          )}
        </>
      )}
    </div>
  )
}
