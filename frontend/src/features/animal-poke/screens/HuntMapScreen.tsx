import { useEffect, useMemo } from 'react'
import PageTitle from '../components/PageTitle'
import DiscoveryPin from '../components/DiscoveryPin'
import { useLbs } from '../../../lbs/useLbs'
import { accuracyCircleRadiusM } from '../../../outdoorSafety/useOutdoorSafety'
import { MAX_ACCURACY_M } from '../../../outdoorSafety/constants'
import { DISCOVERY_RANGE_M } from '../../../lbs/constants'
import { discoveryToHuntTarget } from '../lbsMap'
import type { HuntTarget } from '../data/types'
import { useI18n } from '../../../i18n'
import { chineseRarityName, chineseSpeciesName } from '../petLocalization'

interface HuntMapScreenProps {
  selectedTargetId: string
  onSelectTarget: (id: string) => void
  onBack: () => void
}

export default function HuntMapScreen({
  selectedTargetId,
  onSelectTarget,
  onBack,
}: HuntMapScreenProps) {
  const lbs = useLbs()
  const { t } = useI18n()
  const { state, requestLocation, refreshPoints, nextRefreshIn } = lbs
  const accuracy = state.playerLocation?.accuracy
  const accuracyRadius = accuracyCircleRadiusM(accuracy)
  const accuracyTooLow = accuracy != null && accuracy > MAX_ACCURACY_M

  const circlePercent = useMemo(() => {
    if (accuracyRadius <= 0) return 0
    const pct = (accuracyRadius / DISCOVERY_RANGE_M) * 90
    return Math.max(8, Math.min(pct, 80))
  }, [accuracyRadius])

  useEffect(() => {
    if (state.geoStatus === 'idle' || state.geoStatus === 'denied') {
      requestLocation()
    }
  }, [state.geoStatus, requestLocation])

  const targets: HuntTarget[] = useMemo(() => {
    if (!state.playerLocation) return []
    return state.discoveryPoints
      .filter((p) => p.status !== 'expired')
      .map((p) => discoveryToHuntTarget(p, state.playerLocation))
  }, [state.discoveryPoints, state.playerLocation])

  const selected =
    targets.find((t) => t.id === selectedTargetId) || targets[0] || null

  const minutes = String(Math.floor(Math.max(0, nextRefreshIn) / 60)).padStart(2, '0')
  const secs = String(Math.max(0, nextRefreshIn) % 60).padStart(2, '0')

  const statusLine = (() => {
    if (state.geoStatus === 'locating') return t('map.locating')
    if (state.geoStatus === 'denied') return t('map.denied')
    if (state.geoStatus === 'timeout') return t('map.timeout')
    if (state.geoStatus === 'unsupported') return t('map.unsupported')
    if (!state.playerLocation) return t('map.waiting')
    const acc = state.playerLocation.accuracy
    const accText = typeof acc === 'number' ? `±${Math.round(acc)}m` : t('map.accuracy_unknown')
    return t('map.status', { city: state.cityName || t('map.accuracy_unknown'), accuracy: accText, count: targets.length })
  })()

  return (
    <div className="ap-screen ap-screen--map">
      <button className="ap-map-back" onClick={onBack} type="button">
        {t('map.back')}
      </button>

      <PageTitle
        title={t('map.page_title')}
        subtitle={statusLine}
        rightText={t('map.refreshing', { time: `${minutes}:${secs}` })}
        rightTone="blue"
      />

      <div className="ap-map-canvas" role="region" aria-label={t('map.title')}>
        <div className="ap-road ap-road--blue" />
        <div className="ap-road ap-road--olive" />

        {circlePercent > 0 && (
          <div
            aria-label={t('map.accuracy_radius', { meters: Math.round(accuracy ?? 0) })}
            style={{
              position: 'absolute',
              left: '50%',
              top: '50%',
              width: `${circlePercent}%`,
              height: `${circlePercent}%`,
              transform: 'translate(-50%, -50%)',
              borderRadius: '50%',
              background: accuracyTooLow
                ? 'rgba(244, 67, 54, 0.12)'
                : 'rgba(33, 150, 243, 0.15)',
              border: accuracyTooLow
                ? '1.5px dashed rgba(244, 67, 54, 0.6)'
                : '1.5px solid rgba(33, 150, 243, 0.45)',
              pointerEvents: 'none',
              zIndex: 1,
            }}
          />
        )}

        {targets.map((target) => (
          <DiscoveryPin
            key={target.id}
            target={target}
            selected={target.id === selectedTargetId}
            onSelect={() => onSelectTarget(target.id)}
          />
        ))}

        <div
          className="ap-pin ap-pin--user"
          style={{ left: '50%', top: '50%', zIndex: 2 }}
          role="img"
          aria-label={t('map.you')}
        />
        <div
          className="ap-pin-label"
          style={{
            left: '50%',
            top: 'calc(50% + 18px)',
            transform: 'translateX(-50%)',
            position: 'absolute',
            zIndex: 2,
          }}
        >
          {t('map.you')}
          {accuracy != null ? ` · ±${Math.round(accuracy)}m` : ''}
        </div>

        <div className="ap-map-card">
          {accuracyTooLow && <p>{t('map.accuracy_low')}</p>}
          {state.geoStatus === 'denied' || state.geoStatus === 'unsupported' ? (
            <>
              <h2>{t('map.unavailable')}</h2>
              <p>{state.errorMsg || t('map.permission_prompt')}</p>
              <button type="button" className="ap-map-chip" onClick={() => requestLocation()}>
                {t('map.relocate')}
              </button>
            </>
          ) : !selected ? (
            <>
              <h2>{t('map.no_targets')}</h2>
              <p>{t('map.no_targets_body')}</p>
              <button type="button" className="ap-map-chip" onClick={() => refreshPoints()}>
                {t('map.manual_refresh')}
              </button>
            </>
          ) : (
            <>
              <h2>
                {chineseSpeciesName(selected.species)} · {selected.distanceMeters} 米 · {chineseRarityName(selected.rarity)}
              </h2>
              <p>{t('map.target_detail', { label: selected.label })}</p>
            </>
          )}
        </div>
      </div>
      <section className="ap-map-target-list" aria-label="附近可探索目标">
        <h2>附近可探索目标</h2>
        {targets.length === 0 ? (
          <p>当前没有可探索目标。</p>
        ) : (
          <ul>
            {targets.map((target) => (
              <li key={target.id}>
                {target.label}：距离 {target.distanceMeters} 米，{chineseRarityName(target.rarity)}
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  )
}
