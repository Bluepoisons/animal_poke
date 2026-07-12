import { useEffect, useState, type RefObject } from 'react'
import type { ScreenId } from '../data/types'
import TopResourceBar from '../components/TopResourceBar'
import ActionButton from '../components/ActionButton'
import DetectionOverlay from '../components/DetectionOverlay'
import { useCamera } from '../../../camera/useCamera'
import { guidanceForStatus } from '../../../camera/cameraStatus'
import { usePerfMode } from '../../../performance'
import { compressImageForUpload } from '../../../performance'
import { detectAnimals } from '../../../services/visionDetect'
import type { CaptureFlowState, DetectedAnimal } from '../captureFlow'
import type { CaptureFlowEvent } from '../captureFlow'
import WelfareNotice from '../components/WelfareNotice'
import { canStartScan, localFrameQualityGate, recordScanAttempt, scanModeCopy, loadScanBudget } from '../scanBudget'
import {
  OBSERVATION_TIP_KEYS,
  tipForErrorCode,
  visualStateForFlow,
} from '../recognition/qualityGuidance'
import { useI18n } from '../../../i18n'
import { useSettings } from '../../../settings'

interface DiscoverScreenProps {
  energy: number
  coins: number
  flow: CaptureFlowState
  dispatch: (event: CaptureFlowEvent) => void
  onNavigate: (screen: ScreenId) => void
  onEnterCapture: () => void
  onOpenAccount?: () => void
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

/** Neutral training illustration — never a species detection result (AP-064). */
function TrainingPlaceholder({ label }: { label: string }) {
  return (
    <div className="ap-camera-placeholder" data-testid="camera-placeholder" role="img" aria-label={label}>
      <svg className="ap-camera-placeholder__art" viewBox="0 0 120 120" aria-hidden="true">
        <rect x="18" y="28" width="84" height="64" rx="12" fill="rgba(111,163,210,0.25)" stroke="#2B2B2B" strokeWidth="2.5" />
        <circle cx="60" cy="60" r="18" fill="none" stroke="#2B2B2B" strokeWidth="2.5" />
        <circle cx="60" cy="60" r="8" fill="rgba(255,158,198,0.55)" stroke="#2B2B2B" strokeWidth="2" />
        <rect x="78" y="36" width="14" height="10" rx="3" fill="#F2E66B" stroke="#2B2B2B" strokeWidth="2" />
        <path d="M36 96h48" stroke="#2B2B2B" strokeWidth="2.5" strokeLinecap="round" opacity="0.35" />
      </svg>
      <span className="ap-camera-placeholder__badge">{label}</span>
    </div>
  )
}

export default function DiscoverScreen({
  energy,
  coins,
  flow,
  dispatch,
  onNavigate,
  onEnterCapture,
  onOpenAccount,
  city,
  weather = '—',
}: DiscoverScreenProps) {
  const camera = useCamera()
  const { decision: perf, shouldPauseCamera } = usePerfMode()
  const { t } = useI18n()
  const { settings } = useSettings()
  const speciesLabel = (species: string) =>
    species === 'cat' ? t('species.cat') : species === 'dog' ? t('species.dog') : species === 'goose' ? t('species.goose') : species
  // perf.scanMode: continuous | manual; compress before upload via compressImageForUpload
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    void camera.start()
    return () => camera.stop()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const camGuide = guidanceForStatus(camera.status, camera.error)

  useEffect(() => {
    if (camera.status === 'ready') {
      dispatch({ type: 'CAMERA_READY' })
    } else if (camera.status === 'denied') {
      dispatch({ type: 'CAMERA_ERROR', code: 'camera_denied', message: t(camGuide.reasonKey as never) })
    } else if (camera.status === 'busy') {
      dispatch({ type: 'CAMERA_ERROR', code: 'camera_busy', message: t(camGuide.reasonKey as never) })
    } else if (camera.status === 'unavailable' || camera.status === 'insecure') {
      dispatch({ type: 'CAMERA_ERROR', code: 'camera_unavailable', message: t(camGuide.reasonKey as never) })
    } else if (camera.status === 'ended') {
      dispatch({ type: 'CAMERA_ERROR', code: 'camera_ended', message: t(camGuide.reasonKey as never) })
    }
  }, [camera.status, camera.error, camGuide.reasonKey, dispatch, t])

  const multiSelect =
    flow.errorCode === 'need_select_target' && flow.detections.length > 1
  const recognitionVisual = visualStateForFlow({
    phase: flow.phase,
    errorCode: flow.errorCode,
    hasDetections: flow.detections.length > 0,
    multiSelect,
    targetConfirmed: flow.phase === 'target_confirmed',
    detecting: busy || flow.phase === 'detecting',
  })
  const qualityTip =
    flow.phase === 'failed' || flow.errorCode
      ? tipForErrorCode(flow.errorCode === 'need_select_target' ? 'low_confidence' : flow.errorCode)
      : null

  const statusText = (() => {
    if (recognitionVisual === 'selectable') {
      return t('recognition.state.selectable')
    }
    if (recognitionVisual === 'processing') {
      return t('recognition.state.processing')
    }
    if (flow.phase === 'target_confirmed' && flow.selectedBox) {
      const s = flow.selectedBox
      return `${t('recognition.state.ready')} · ${speciesLabel(s.species)} ${Math.round(s.confidence * 100)}%`
    }
    if (recognitionVisual === 'error' || recognitionVisual === 'low_confidence') {
      if (qualityTip) {
        return `${t(qualityTip.titleKey as never)} · ${t(qualityTip.bodyKey as never)}`
      }
      return flow.errorMessage || t('recognition.state.error')
    }
    if (camera.status === 'ready') {
      const b = loadScanBudget()
      const facingLabel =
        camera.facing === 'user' ? t('camera.facing.user') : t('camera.facing.environment')
      return `${t('camera.status.ready')} · ${facingLabel} · ${scanModeCopy(b.mode)} · 今日剩余 ${Math.max(0, b.dailyQuota - b.usedToday)}`
    }
    // Every non-ready CameraStatus: reason + next step
    return `${t(camGuide.reasonKey as never)} · ${t(camGuide.nextKey as never)}`
  })()

  const showSettingsHelp = camera.status === 'denied' || camera.status === 'insecure'
  const showRetry =
    camera.status === 'busy' ||
    camera.status === 'ended' ||
    camera.status === 'stopped' ||
    camera.status === 'unavailable' ||
    camera.status === 'idle' ||
    camera.status === 'denied'

  const videoEl = camera.videoRef.current
  const videoWidth = videoEl?.videoWidth || 640
  const videoHeight = videoEl?.videoHeight || 480
  // Prefer in-frame selection for multi-target; always show boxes when we have detections
  // after a successful detect (including single auto-select / confirmed).
  const showBoxes =
    flow.detections.length > 0 &&
    (flow.phase === 'detecting' ||
      flow.phase === 'target_confirmed' ||
      multiSelect ||
      flow.errorCode === 'need_select_target') &&
    // Avoid overlay while primary CTA is the only action and camera is not live
    (camGuide.livePreview || multiSelect)


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

    const gate = canStartScan()
    if (!gate.ok) {
      dispatch({
        type: 'DETECT_FAIL',
        code: gate.reason,
        message:
          gate.reason === 'quota_exhausted'
            ? `今日识别次数已用完（${gate.state.dailyQuota}）`
            : gate.reason === 'too_fast'
              ? '扫描过快，请稍候再试'
              : '当前仅允许手动扫描',
      })
      return
    }

    const blob = await camera.captureFrame()
    if (!blob) {
      dispatch({ type: 'DETECT_FAIL', code: 'no_frame', message: '未获取到有效画面' })
      return
    }

    const quality = await localFrameQualityGate(blob)
    if (!quality.ok) {
      dispatch({
        type: 'DETECT_FAIL',
        code: quality.reason || 'quality',
        message:
          quality.reason === 'duplicate_frame'
            ? '画面几乎未变化，请调整角度后再扫'
            : '画面质量不足，请靠近并保持稳定',
      })
      return
    }

    setBusy(true)
    recordScanAttempt()
    dispatch({ type: 'START_DETECT', photoBlob: blob })
    try {
      const result = await detectAnimals(blob)
      const detections: DetectedAnimal[] = result.animals.map((a, i) => ({
        ...a,
        id: a.targetId || `det-${i}-${a.species}`,
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
      ? t('discover.enter_capture')
      : busy || flow.phase === 'detecting'
        ? t('discover.detecting')
        : t('discover.start')

  const onPrimary = () => {
    if (flow.phase === 'target_confirmed') {
      handleEnterCapture()
      return
    }
    void handleScan()
  }

  return (
    <div className="ap-screen" data-testid="discover-screen">
      <TopResourceBar city={city || t('discover.city_loading')} weather={weather} energy={energy} coins={coins} />
      {settings.homeMode && (
        <div
          role="status"
          data-testid="home-mode-banner"
          style={{
            margin: '8px 0',
            padding: 10,
            borderRadius: 12,
            background: '#E8F5E9',
            color: '#1B5E20',
            fontSize: 13,
          }}
        >
          {t('settings.homeMode.hint')}
        </div>
      )}

      <div className="ap-discover__hero">
        <div className="ap-discover__eyebrow-row">
          <div className="ap-discover__eyebrow">DISCOVER MODE</div>
          <div style={{ display: 'flex', gap: 8 }}>
            <button
              type="button"
              className="ap-account-entry"
              onClick={() => onNavigate('settings')}
              data-testid="open-settings"
            >
              {t('discover.settings')}
            </button>
            {onOpenAccount ? (
              <button type="button" className="ap-account-entry" onClick={onOpenAccount} data-testid="open-account">
                {t('discover.account')}
              </button>
            ) : null}
          </div>
        </div>
        <h1 className="ap-discover__title">
          <span className="ap-highlight ap-highlight--pink">{t('discover.title_line_one')}</span>
          <br />
          {t('discover.title_line_two')}
        </h1>
      </div>

      <div className="ap-discover__map-row">
        <button
          className="ap-map-chip"
          onClick={() => onNavigate('map')}
          type="button"
          aria-label={t('discover.open_map_label')}
        >
          {t('discover.open_map')}
        </button>
      </div>

      <div className="ap-scan-stage">
        <DoodleStar />
        <DoodleHeart />
        <div
          className="ap-scan-box"
          data-camera-status={camera.status}
          data-live-preview={camGuide.livePreview ? 'true' : 'false'}
          data-recognition={recognitionVisual}
        >
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
            data-testid="camera-video"
            style={{
              position: 'absolute',
              inset: 0,
              width: '100%',
              height: '100%',
              objectFit: 'cover',
              opacity: camGuide.livePreview ? 1 : 0,
              // Mirror front camera for natural selfie framing
              transform: camera.facing === 'user' && camGuide.livePreview ? 'scaleX(-1)' : undefined,
            }}
          />
          {!camGuide.livePreview && (
            <TrainingPlaceholder
              label={
                camGuide.placeholderKind === 'unavailable'
                  ? t('camera.placeholder.unavailable')
                  : t('camera.placeholder.training')
              }
            />
          )}
          {camGuide.livePreview && recognitionVisual === 'processing' && (
            <div className="ap-scan-line" data-testid="scan-line-processing" />
          )}
          {camGuide.livePreview && !showBoxes && recognitionVisual === 'idle' && <MascotBlob />}
          {showBoxes && (
            <DetectionOverlay
              detections={flow.detections}
              selectedId={flow.selectedBox?.id ?? null}
              videoWidth={videoWidth}
              videoHeight={videoHeight}
              mirrorX={camera.facing === 'user' && camGuide.livePreview}
              objectFit="cover"
              speciesLabel={speciesLabel}
              visualState={
                recognitionVisual === 'idle' ? 'selectable' : (recognitionVisual as never)
              }
              onSelect={(animalId) => dispatch({ type: 'SELECT_TARGET', animalId })}
            />
          )}
        </div>
      </div>

      <div className="ap-camera-toolbar" data-testid="camera-toolbar">
        <button
          type="button"
          className="ap-map-chip"
          data-testid="camera-switch"
          disabled={camera.status === 'requesting'}
          onClick={() => void camera.switchFacing()}
          aria-label={
            camera.facing === 'environment'
              ? t('camera.action.switch_to_front')
              : t('camera.action.switch_to_back')
          }
        >
          {camera.status === 'ready'
            ? camera.facing === 'environment'
              ? t('camera.action.switch_to_front')
              : t('camera.action.switch_to_back')
            : t('camera.action.switch')}
        </button>
        {showRetry && (
          <button
            type="button"
            className="ap-map-chip"
            data-testid="camera-retry"
            onClick={() => void camera.retry()}
          >
            {t('camera.action.retry')}
          </button>
        )}
        {showSettingsHelp && (
          <button
            type="button"
            className="ap-map-chip"
            data-testid="camera-settings-help"
            onClick={() => {
              // Browsers cannot deep-link to OS camera settings; surface explicit guidance.
              window.alert(
                camera.status === 'insecure'
                  ? t('camera.next.insecure')
                  : t('camera.next.denied'),
              )
            }}
          >
            {t('camera.action.open_settings')}
          </button>
        )}
      </div>

      <div
        className="ap-result-pill"
        role="status"
        aria-live="polite"
        data-testid="camera-status-pill"
        data-recognition={recognitionVisual}
      >
        <span
          className="ap-result-pill__dot"
          data-status={
            recognitionVisual === 'error' || recognitionVisual === 'low_confidence'
              ? 'denied'
              : recognitionVisual === 'processing'
                ? 'requesting'
                : recognitionVisual === 'ready_capture'
                  ? 'ready'
                  : camera.status
          }
          aria-hidden="true"
        />
        <span>{statusText}</span>
      </div>

      {qualityTip && (recognitionVisual === 'error' || recognitionVisual === 'low_confidence') && (
        <div
          className={`ap-quality-tip ap-quality-tip--${qualityTip.tone}`}
          data-testid="quality-tip"
          role="alert"
        >
          <strong>{t(qualityTip.titleKey as never)}</strong>
          <p>{t(qualityTip.bodyKey as never)}</p>
        </div>
      )}

      <p className="ap-photo-skill-hint" role="note">
        {t('discover.photo_hint')}
      </p>

      <ul className="ap-observe-tips" data-testid="observe-tips">
        {OBSERVATION_TIP_KEYS.map((key) => (
          <li key={key}>{t(key as never)}</li>
        ))}
      </ul>

      {showSelect && (
        <div
          style={{ display: 'flex', flexWrap: 'wrap', gap: 8, padding: '0 16px 8px' }}
          data-testid="detect-chip-row"
        >
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
              {speciesLabel(d.species)} {Math.round(d.confidence * 100)}%
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
            {t('discover.confirm_target')}
          </button>
        </div>
      )}


      {flow.phase === 'target_confirmed' && flow.selectedBox && (
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, padding: '0 16px 8px' }}>
          <button
            type="button"
            className="ap-map-chip"
            onClick={() => {
              dispatch({ type: 'RESET' })
            }}
          >
            {t('discover.retry_scan')}
          </button>
          {(['cat', 'dog', 'goose'] as const)
            .filter((s) => s !== flow.selectedBox?.species)
            .map((s) => (
              <button
                key={s}
                type="button"
                className="ap-map-chip"
                onClick={() => {
                  // 纠正：用玩家声明物种覆盖 selectedBox
                  const corrected = {
                    ...flow.selectedBox!,
                    species: s,
                    id: `correct-${s}`,
                    confidence: Math.min(flow.selectedBox!.confidence, 0.84),
                    label: `user_correct:${s}`,
                  }
                  dispatch({ type: 'DETECT_SUCCESS', detectInferenceId: flow.detectInferenceId || 'user-correct', detections: [corrected] })
                }}
              >
                {t('discover.correct_to', { species: speciesLabel(s) })}
              </button>
            ))}
        </div>
      )}

      {flow.phase === 'failed' && (
        <button
          type="button"
          className="ap-map-chip"
          style={{ margin: '0 16px 8px' }}
          onClick={() => dispatch({ type: 'RESET' })}
        >
          {t('discover.restart')}
        </button>
      )}

      <WelfareNotice />

      <ActionButton
        onClick={onPrimary}
        data-testid={
          flow.phase === 'target_confirmed'
            ? 'enter-capture'
            : busy || flow.phase === 'detecting'
              ? 'detect-busy'
              : 'start-detect'
        }
        disabled={
          // Never block Enter Capture once a target is confirmed (E2E hard gate).
          flow.phase === 'target_confirmed'
            ? false
            : busy || (flow.phase !== 'failed' && !canScan)
        }
      >
        {primaryLabel}
      </ActionButton>
    </div>
  )
}
