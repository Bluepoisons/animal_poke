import { useEffect, useState } from 'react'
import PageTitle from '../components/PageTitle'
import DiscoveryPin from '../components/DiscoveryPin'
import { huntTargets, getTargetById } from '../data/huntTargets'

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
          style={{ left: '50%', top: '50%' }}
          aria-label="你的位置"
        />
        <div
          className="ap-pin-label"
          style={{
            left: '50%',
            top: 'calc(50% + 18px)',
            transform: 'translateX(-50%)',
            position: 'absolute',
          }}
        >
          你的位置
        </div>

        <div className="ap-map-card">
          <h2>
            {speciesNames[selected.species]} · {selected.label}
          </h2>
          <p>500m 范围内 7 个目标，诱饵会提升稀有出现率。</p>
        </div>
      </div>
    </div>
  )
}
