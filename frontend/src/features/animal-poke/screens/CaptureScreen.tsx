import { useState } from 'react'
import PageTitle from '../components/PageTitle'
import AnimalIcon from '../components/AnimalIcon'
import CaptureProbabilityBar from '../components/CaptureProbabilityBar'

interface CaptureScreenProps {
  onToast: (message: string) => void
}

export default function CaptureScreen({ onToast }: CaptureScreenProps) {
  const [power] = useState(55)
  const bestMin = 35
  const bestMax = 75
  const captureRate = 0.78

  const handleCapture = () => {
    if (power >= bestMin && power <= bestMax) {
      onToast('捕获成功：鹅已加入图鉴')
    } else {
      onToast('捕获失败，再试一次')
    }
  }

  return (
    <div className="ap-screen">
      <PageTitle
        title="CAPTURE"
        subtitle="点击画面投掷 · 手账捕捉页"
        rightText="体力 -20"
        rightTone="pink"
      />

      <div
        className="ap-capture-stage"
        onClick={handleCapture}
        onKeyDown={(event) => {
          if (event.key === 'Enter' || event.key === ' ') {
            event.preventDefault()
            handleCapture()
          }
        }}
        role="button"
        tabIndex={0}
        aria-label="投掷捕获"
      >
        <div className="ap-capture-target">
          <div className="ap-animal-badge ap-animal-badge--yellow" style={{ width: 148, height: 148 }}>
            <AnimalIcon species="goose" size={112} />
          </div>
        </div>
        <div className="ap-throw-line" aria-hidden="true" />
        <div className="ap-capture-item" aria-hidden="true">
          <svg width="34" height="34" viewBox="0 0 80 80">
            <circle cx="40" cy="40" r="28" fill="#FFFDF8" stroke="#2B2B2B" strokeWidth="5" />
            <path d="M12 40h56" stroke="#2B2B2B" strokeWidth="5" />
            <circle cx="40" cy="40" r="10" fill="#FF9EC6" stroke="#2B2B2B" strokeWidth="4" />
          </svg>
        </div>
      </div>

      <CaptureProbabilityBar
        title="鹅 · 面包屑球 · 弹跳略强"
        successRate={captureRate}
        bestMin={bestMin}
        bestMax={bestMax}
      />

      <p className="ap-capture-hint">轻点画面投出贴纸球</p>
    </div>
  )
}
