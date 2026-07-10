import { useEffect, useMemo, useState } from 'react'
import PageTitle from '../components/PageTitle'
import DiscoveryPin from '../components/DiscoveryPin'
import { huntTargets, getTargetById } from '../data/huntTargets'
import { useLbs } from '../../../lbs/useLbs'
import { accuracyCircleRadiusM } from '../../../outdoorSafety/useOutdoorSafety'
import { MAX_ACCURACY_M } from '../../../outdoorSafety/constants'
import { DISCOVERY_RANGE_M } from '../../../lbs/constants'

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
  const [seconds, setSeconds] = useState(272)
  const selected = getTargetById(selectedTargetId) ?? huntTargets[2]
  const lbs = useLbs()
  const accuracy = lbs.state.playerLocation?.accuracy
  const accuracyRadius = accuracyCircleRadiusM(accuracy)
  const accuracyTooLow = accuracy != null && accuracy > MAX_ACCURACY_M

  const circlePercent = useMemo(() => {
    if (accuracyRadius <= 0) return 0
    const pct = (accuracyRadius / DISCOVERY_RANGE_M) * 90
    return Math.max(8, Math.min(pct, 80))
  }, [accuracyRadius])

  useEffect(() => {
    const interval = window.setInterval(() => {
      setSeconds((prev) => {
        if (prev <= 0) return 272
        return prev - 1
      })
    }, 1000)
    return () => window.clearInterval(interval)
  }, [])

  const minutes = String(Math.floor(seconds / 60)).padStart(2, '0')
  const secs = String(seconds % 60).padStart(2, '0')

  return (
    <div className="ap-screen ap-screen--map">
      <button className="ap-map-back" onClick={onBack} type="button">
        返回手账
      </button>

      <PageTitle
        title="HUNT MAP"
        subtitle="附近发现点 · 手绘地图"
        rightText={`刷新 ${minutes}:${secs}`}
        rightTone="blue"
      />

      <div className="ap-map-canvas" aria-label="猎取地图">
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

        {huntTargets.map((target) => (
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
          <h2>
            {speciesNames[selected.species]} · {selected.label}
          </h2>
          <p>
            500m 范围内 7 个目标，诱饵会提升稀有出现率。
            {accuracyTooLow
              ? ' 定位精度不足，无法判定进入捕获范围。'
              : ''}
          </p>
        </div>
      </div>
    </div>
  )
}
