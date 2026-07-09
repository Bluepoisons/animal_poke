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

function DoodleStar() {
  return (
    <svg className="ap-doodle ap-doodle--star" width="28" height="28" viewBox="0 0 28 28" aria-hidden="true">
      <path
        d="M14 2l3.2 7.4L25 12l-7 4.2L16.8 26 14 20.4 11.2 26 10 16.2 3 12l7.8-2.6L14 2Z"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinejoin="round"
      />
    </svg>
  )
}

function DoodleHeart() {
  return (
    <svg className="ap-doodle ap-doodle--heart" width="26" height="24" viewBox="0 0 26 24" aria-hidden="true">
      <path
        d="M13 21s-8.5-5.2-11-9.4C.2 8.4 1.6 4 5.8 3.4c2.4-.4 4.4.8 5.6 2.4C12.6 4.2 14.6 3 17 3.4c4.2.6 5.6 5 3.8 8.2C18.4 15.8 13 21 13 21Z"
        fill="rgba(255,158,198,0.35)"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinejoin="round"
      />
    </svg>
  )
}

function MascotBlob() {
  return (
    <svg className="ap-mascot" viewBox="0 0 72 72" aria-hidden="true">
      <ellipse cx="36" cy="40" rx="24" ry="22" fill="rgba(255,158,198,0.55)" stroke="#2B2B2B" strokeWidth="2.5" />
      <circle cx="28" cy="36" r="3" fill="#2B2B2B" />
      <circle cx="44" cy="36" r="3" fill="#2B2B2B" />
      <path d="M30 46c4 4 10 4 14 0" fill="none" stroke="#2B2B2B" strokeWidth="2.5" strokeLinecap="round" />
      <path d="M20 24c2-8 8-10 12-6M52 24c-2-8-8-10-12-6" fill="none" stroke="#2B2B2B" strokeWidth="2.5" strokeLinecap="round" />
      <circle cx="54" cy="18" r="5" fill="#F2E66B" stroke="#2B2B2B" strokeWidth="2" />
    </svg>
  )
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

      <div className="ap-discover__hero">
        <div className="ap-discover__eyebrow">DISCOVER MODE</div>
        <h1 className="ap-discover__title">
          <span className="ap-highlight ap-highlight--pink">VLM 实时</span>
          <br />
          动物识别中
        </h1>
      </div>

      <div className="ap-discover__map-row">
        <button
          className="ap-map-chip"
          onClick={() => onNavigate('map')}
          type="button"
          aria-label="打开地图"
        >
          打开猎取地图
        </button>
      </div>

      <div className="ap-scan-stage">
        <DoodleStar />
        <DoodleHeart />
        <div className="ap-scan-box">
          <div className="ap-scan-box__corners" aria-hidden="true">
            <span />
            <span />
            <span />
            <span />
          </div>
          <AnimalIcon species="goose" size={108} />
          <div className="ap-scan-line" />
          <MascotBlob />
        </div>
      </div>

      <div className="ap-result-pill">
        <span className="ap-result-pill__dot" aria-hidden="true" />
        <span>鹅 · 置信度 94%</span>
      </div>

      <ActionButton onClick={onStartCapture}>开始捕获</ActionButton>
    </div>
  )
}
