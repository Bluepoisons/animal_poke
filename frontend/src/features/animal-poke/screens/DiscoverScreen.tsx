import type { ScreenId } from '../data/types'
import TopResourceBar from '../components/TopResourceBar'
import ActionButton from '../components/ActionButton'
import AnimalIcon from '../components/AnimalIcon'

interface DiscoverScreenProps {
  energy: number
  coins: number
  onStartCapture: () => void
  onNavigate: (screen: ScreenId) => void
}

export default function DiscoverScreen({
  energy,
  coins,
  onStartCapture,
  onNavigate,
}: DiscoverScreenProps) {
  return (
    <div className="ap-screen">
      <TopResourceBar city="宁波" weather="雨" energy={energy} coins={coins} />
      <div className="ap-discover__eyebrow">DISCOVER MODE</div>
      <h1 className="ap-discover__title">VLM 实时动物识别中</h1>
      <button
        className="ap-discover__map-button"
        onClick={() => onNavigate('map')}
        type="button"
        aria-label="打开地图"
      >
        地图
      </button>
      <div className="ap-discover__ambient" />
      <div className="ap-scan-box">
        <AnimalIcon species="goose" size={96} />
        <div className="ap-scan-line" />
      </div>
      <div className="ap-result-pill">
        <span>🪿</span>
        <span>鹅 · 置信度 94%</span>
      </div>
      <div className="ap-lower-band" />
      <ActionButton onClick={onStartCapture}>开始捕获</ActionButton>
    </div>
  )
}
