import { useEffect, useMemo } from 'react'
import PageTitle from '../components/PageTitle'
import DiscoveryPin from '../components/DiscoveryPin'
import { useLbs } from '../../../lbs/useLbs'
import { accuracyCircleRadiusM } from '../../../outdoorSafety/useOutdoorSafety'
import { MAX_ACCURACY_M } from '../../../outdoorSafety/constants'
import { DISCOVERY_RANGE_M } from '../../../lbs/constants'
import { discoveryToHuntTarget } from '../lbsMap'
import type { HuntTarget } from '../data/types'

interface HuntMapScreenProps {
  selectedTargetId: string
  onSelectTarget: (id: string) => void
  onBack: () => void
}

const speciesNames = {
  goose: '鹅',
  cat: '猫',
  dog: '狗',
} as const

export default function HuntMapScreen({
  selectedTargetId,
  onSelectTarget,
  onBack,
}: HuntMapScreenProps) {
  const lbs = useLbs()
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
    if (state.geoStatus === 'locating') return '定位中…'
    if (state.geoStatus === 'denied') return '定位被拒绝'
    if (state.geoStatus === 'timeout') return '定位超时'
    if (state.geoStatus === 'unsupported') return '设备不支持定位'
    if (!state.playerLocation) return '等待定位'
    const acc = state.playerLocation.accuracy
    const accText = typeof acc === 'number' ? `±${Math.round(acc)}m` : '精度未知'
    return `${state.cityName || '未知城市'} · ${accText} · ${targets.length} 个点`
  })()

  return (
    <div className="ap-screen ap-screen--map">
      <button className="ap-map-back" onClick={onBack} type="button">
        返回手账
      </button>

      <PageTitle
        title="HUNT MAP"
        subtitle={statusLine}
        rightText={`刷新 ${minutes}:${secs}`}
        rightTone="blue"
      />

      <div className="ap-map-canvas" role="region" aria-label="猎取地图">
        <div className="ap-road ap-road--blue" />
        <div className="ap-road ap-road--olive" />

        {circlePercent > 0 && (
          <div
            aria-label={`定位精度约 ${Math.round(accuracy ?? 0)} 米`}
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
          aria-label="你的位置"
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
          你的位置
          {accuracy != null ? ` · ±${Math.round(accuracy)}m` : ''}
        </div>

        <div className="ap-map-card">
          {accuracyTooLow && <p>定位精度不足，无法判定进入捕获范围。</p>}
          {state.geoStatus === 'denied' || state.geoStatus === 'unsupported' ? (
            <>
              <h2>无法定位</h2>
              <p>{state.errorMsg || '请开启定位权限后重试。'}</p>
              <button type="button" className="ap-map-chip" onClick={() => requestLocation()}>
                重新定位
              </button>
            </>
          ) : !selected ? (
            <>
              <h2>附近暂无发现点</h2>
              <p>定位成功后会生成可探索目标。低精度或极端天气时可能为空。</p>
              <button type="button" className="ap-map-chip" onClick={() => refreshPoints()}>
                手动刷新
              </button>
            </>
          ) : (
            <>
              <h2>
                {speciesNames[selected.species as keyof typeof speciesNames] ?? selected.species} · {selected.distanceMeters}m · {selected.rarity}
              </h2>
              <p>
                {selected.label}。超出捕获范围时无法开始捕获；服务端权威校验见后续接口。
              </p>
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
                {target.label}：{speciesNames[target.species as keyof typeof speciesNames]}，距离 {target.distanceMeters} 米，{target.rarity}
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  )
}
