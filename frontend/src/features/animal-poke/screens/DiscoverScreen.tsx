import { useEffect, useState, type RefObject } from 'react'
import type { ScreenId } from '../data/types'
import TopResourceBar from '../components/TopResourceBar'
import ActionButton from '../components/ActionButton'
import AnimalIcon from '../components/AnimalIcon'
import { useCamera } from '../../../camera/useCamera'
import { detectAnimals } from '../../../services/visionDetect'
import type { CaptureFlowState, DetectedAnimal } from '../captureFlow'
import type { CaptureFlowEvent } from '../captureFlow'

interface DiscoverScreenProps {
  energy: number
  coins: number
  flow: CaptureFlowState
  dispatch: (event: CaptureFlowEvent) => void
  onNavigate: (screen: ScreenId) => void
  onEnterCapture: () => void
  city?: string
  weather?: string
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

const SPECIES_LABEL: Record<string, string> = {
  cat: '猫',
  dog: '狗',
  goose: '鹅',
}

export default function DiscoverScreen({
  energy,
  coins,
  flow,
  dispatch,
  onNavigate,
  onEnterCapture,
  city = '定位中',
  weather = '—',
}: DiscoverScreenProps) {
  const camera = useCamera()
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    void camera.start()
    return () => camera.stop()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    if (camera.status === 'ready') {
      dispatch({ type: 'CAMERA_READY' })
    } else if (camera.status === 'denied') {
      dispatch({ type: 'CAMERA_ERROR', code: 'camera_denied', message: '相机权限被拒绝' })
    } else if (camera.status === 'busy') {
      dispatch({ type: 'CAMERA_ERROR', code: 'camera_busy', message: '相机被占用' })
    } else if (camera.status === 'unavailable') {
      dispatch({ type: 'CAMERA_ERROR', code: 'camera_unavailable', message: '设备无可用相机' })
    }
  }, [camera.status, dispatch])

  const statusText = (() => {
    if (flow.phase === 'detecting' && flow.errorCode === 'need_select_target') {
      return flow.errorMessage || '请选择目标'
    }
    if (flow.phase === 'detecting') return '正在识别…'
    if (flow.phase === 'target_confirmed' && flow.selectedBox) {
      const s = flow.selectedBox
      return `识别：${SPECIES_LABEL[s.species] || s.species} · 置信度 ${Math.round(s.confidence * 100)}%`
    }
    if (flow.phase === 'failed' && flow.errorMessage) return flow.errorMessage
    if (camera.status === 'denied') return '相机权限被拒绝 · 请在系统设置开启'
    if (camera.status === 'busy') return '相机被占用'
    if (camera.status === 'unavailable') return '设备无可用相机 · 使用占位预览'
    if (camera.status === 'requesting') return '正在打开相机…'
    if (camera.status === 'ready') return '相机就绪 · 点击开始识别'
    return '准备相机…'
  })()

  const canScan =
    !busy &&
    camera.status === 'ready' &&
    flow.phase !== 'detecting' &&
    typeof navigator !== 'undefined' &&
    navigator.onLine !== false

  const handleScan = async () => {
    if (!canScan) {
      if (typeof navigator !== 'undefined' && navigator.onLine === false) {
        dispatch({ type: 'DETECT_FAIL', code: 'offline', message: '离线无法识别' })
      } else if (camera.status !== 'ready') {
        dispatch({
          type: 'DETECT_FAIL',
          code: 'camera_not_ready',
          message: '相机未就绪，无法识别',
        })
      }
      return
    }

    const blob = await camera.captureFrame()
    if (!blob) {
      dispatch({ type: 'DETECT_FAIL', code: 'no_frame', message: '未获取到有效画面' })
      return
    }

    setBusy(true)
    dispatch({ type: 'START_DETECT', photoBlob: blob })
    try {
      const result = await detectAnimals(blob)
      const detections: DetectedAnimal[] = result.animals.map((a, i) => ({
        ...a,
        id: `det-${i}-${a.species}`,
      }))
      dispatch({
        type: 'DETECT_SUCCESS',
        detections,
        detectInferenceId: result.inferenceId,
      })
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'detect_failed'
      let code = 'detect_failed'
      if (msg.includes('no_animals')) code = 'no_animals'
      if (msg.includes('401') || msg.includes('403')) code = 'auth_error'
      if (msg.includes('429')) code = 'rate_limited'
      if (msg.includes('Abort') || msg.includes('timeout')) code = 'timeout'
      dispatch({
        type: 'DETECT_FAIL',
        code,
        message:
          code === 'no_animals'
            ? '画面中未发现动物'
            : code === 'auth_error'
              ? '鉴权失败，请稍后重试'
              : code === 'rate_limited'
                ? '请求过于频繁，请稍后再试'
                : code === 'timeout'
                  ? '识别超时'
                  : `识别失败：${msg}`,
      })
    } finally {
      setBusy(false)
    }
  }

  const handleEnterCapture = () => {
    if (flow.phase === 'target_confirmed' && flow.selectedBox) {
      onEnterCapture()
      return
    }
    // multi-select path: confirm first
    if (flow.selectedBox && flow.photoBlob && flow.detectInferenceId) {
      dispatch({ type: 'CONFIRM_TARGET' })
      onEnterCapture()
    }
  }

  const showSelect =
    flow.errorCode === 'need_select_target' && flow.detections.length > 1

  const primaryLabel =
    flow.phase === 'target_confirmed'
      ? '进入捕获'
      : busy || flow.phase === 'detecting'
        ? '识别中…'
        : '开始识别'

  const onPrimary = () => {
    if (flow.phase === 'target_confirmed') {
      handleEnterCapture()
      return
    }
    void handleScan()
  }

  return (
    <div className="ap-screen">
      <TopResourceBar city={city} weather={weather} energy={energy} coins={coins} />

      <div className="ap-discover__hero">
        <div className="ap-discover__eyebrow">DISCOVER MODE</div>
        <h1 className="ap-discover__title">
          <span className="ap-highlight ap-highlight--pink">真实识别</span>
          <br />
          发现野生动物
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
          <video
            ref={camera.videoRef as RefObject<HTMLVideoElement>}
            playsInline
            muted
            autoPlay
            style={{
              position: 'absolute',
              inset: 0,
              width: '100%',
              height: '100%',
              objectFit: 'cover',
              opacity: camera.status === 'ready' ? 1 : 0,
            }}
          />
          {camera.status !== 'ready' && (
            <AnimalIcon species={flow.selectedBox?.species || 'goose'} size={108} />
          )}
          <div className="ap-scan-line" />
          <MascotBlob />
        </div>
      </div>

      <div className="ap-result-pill" role="status" aria-live="polite">
        <span className="ap-result-pill__dot" aria-hidden="true" />
        <span>{statusText}</span>
      </div>

      {showSelect && (
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, padding: '0 16px 8px' }}>
          {flow.detections.map((d) => (
            <button
              key={d.id}
              type="button"
              className="ap-map-chip"
              style={{
                outline: flow.selectedBox?.id === d.id ? '2px solid #FF8C42' : undefined,
              }}
              onClick={() => dispatch({ type: 'SELECT_TARGET', animalId: d.id })}
            >
              {SPECIES_LABEL[d.species] || d.species} {Math.round(d.confidence * 100)}%
            </button>
          ))}
          <button
            type="button"
            className="ap-map-chip"
            disabled={!flow.selectedBox}
            onClick={() => {
              dispatch({ type: 'CONFIRM_TARGET' })
              onEnterCapture()
            }}
          >
            确认目标并捕获
          </button>
        </div>
      )}

      {flow.phase === 'failed' && (
        <button
          type="button"
          className="ap-map-chip"
          style={{ margin: '0 16px 8px' }}
          onClick={() => dispatch({ type: 'RESET' })}
        >
          重新开始
        </button>
      )}

      <ActionButton
        onClick={onPrimary}
        disabled={
          busy ||
          (flow.phase !== 'target_confirmed' && !canScan && flow.phase !== 'failed')
        }
      >
        {primaryLabel}
      </ActionButton>
    </div>
  )
}
