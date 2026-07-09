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
    <div className="ap-screen" onClick={handleCapture} role="button" tabIndex={0} aria-label="投掷捕获">
      <PageTitle title="CAPTURE MODE" rightText="体力 -20" />
      <div className="ap-capture-target">
        <AnimalIcon species="goose" size={112} />
      </div>
      <div className="ap-throw-line" />
      <div className="ap-capture-item">
        <svg
          width="54"
          height="54"
          viewBox="0 0 80 80"
          className="ap-animal ap-animal--light"
          aria-hidden="true"
        >
          <path
            d="M18 57V34c0-17 15-27 31-22 13 4 20 18 10 29v16c0 7-5 12-12 12H30c-7 0-12-5-12-12Z"
            fill="none"
            stroke="currentColor"
            strokeWidth="6"
            strokeLinejoin="round"
          />
          <path d="M18 36c11-10 25-10 41 0" fill="currentColor" />
        </svg>
      </div>
      <CaptureProbabilityBar
        title="鹅 · 面包屑球 · 弹跳略强"
        successRate={captureRate}
        bestMin={bestMin}
        bestMax={bestMax}
      />
    </div>
  )
}
