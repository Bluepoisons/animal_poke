import { useEffect, useMemo, useRef, useState } from 'react'
import PageTitle from '../components/PageTitle'
import AnimalIcon from '../components/AnimalIcon'
import CaptureProbabilityBar from '../components/CaptureProbabilityBar'
import { useStamina } from '../../../stamina/useStamina'
import { useLbs } from '../../../lbs/useLbs'
import { useWeather } from '../../../weather/useWeather'
import SafetyStopBanner from '../../../outdoorSafety/SafetyStopBanner'
import { evaluateOutdoorSafety } from '../../../outdoorSafety/logic'
import {
  createCaptureSession,
  settleCapture,
  BEST_MIN,
  BEST_MAX,
  successProbability,
} from '../../../capture/session'
import type { DetectionResult } from '../../../services/visionDetect'
import type { SpeciesType } from '../../../types'

interface CaptureScreenProps {
  onToast: (message: string) => void
  species: SpeciesType
  detection: DetectionResult
  detectInferenceId: string
  targetId?: string | null
  captureAttemptId: string
  onSettled?: (ok: boolean) => void
  onInvalidAccess?: () => void
}

export default function CaptureScreen({
  onToast,
  species,
  detection,
  detectInferenceId,
  targetId,
  captureAttemptId,
  onSettled,
  onInvalidAccess,
}: CaptureScreenProps) {
  const { state: staminaState, consumeStamina } = useStamina()
  const currentStamina = staminaState.currentStamina
  const lbs = useLbs()
  const weather = useWeather()
  const [power] = useState(55)
  const [battery, setBattery] = useState<{ level: number | null; charging: boolean | null }>({
    level: null,
    charging: null,
  })
  const sessionRef = useRef(
    createCaptureSession({
      species,
      detection,
      targetId: targetId || undefined,
      power,
    }),
  )
  const session = sessionRef.current
  const captureRate = useMemo(() => successProbability(power), [power])

  useEffect(() => {
    if (!detectInferenceId || !detection) {
      onInvalidAccess?.()
    }
  }, [detectInferenceId, detection, onInvalidAccess])

  useEffect(() => {
    let cancelled = false
    const nav = navigator as Navigator & {
      getBattery?: () => Promise<{ level: number; charging: boolean }>
    }
    if (typeof nav.getBattery === 'function') {
      nav.getBattery().then((b) => {
        if (!cancelled) setBattery({ level: b.level, charging: b.charging })
      }).catch(() => {})
    }
    return () => {
      cancelled = true
    }
  }, [])

  const outdoor = useMemo(() => {
    const loc = lbs.state.playerLocation
    return evaluateOutdoorSafety({
      weather: weather.today?.weather ?? null,
      accuracyM: loc?.accuracy ?? null,
      batteryLevel: battery.level,
      batteryCharging: battery.charging,
      speedMps: null,
    })
  }, [lbs.state.playerLocation, weather.today?.weather, battery.level, battery.charging])

  const handleCapture = () => {
    if (!outdoor.allowed) {
      onToast(outdoor.messages[0] ?? '户外捕获已暂停，请先停下再操作')
      return
    }
    const online = typeof navigator === 'undefined' ? true : navigator.onLine
    const result = settleCapture({
      session: sessionRef.current,
      online,
      stamina: currentStamina,
      consumeStamina: (n) => consumeStamina(n),
    })
    sessionRef.current = result.session
    if (result.ok) {
      onToast(
        `捕获成功：${result.session.species} · 置信度 ${Math.round((detection.confidence || 0) * 100)}% · id ${detectInferenceId.slice(0, 8)}`,
      )
      onSettled?.(true)
      return
    }
    if (result.reason === 'already_settled') {
      onToast('本轮已结算')
      return
    }
    if (result.reason === 'offline') {
      onToast('离线无法捕获')
      onSettled?.(false)
      return
    }
    if (result.reason === 'no_stamina') {
      onToast('体力不足')
      return
    }
    onToast('捕获失败，再试一次')
    onSettled?.(false)
  }

  return (
    <div className="ap-screen">
      <PageTitle
        title="CAPTURE"
        subtitle={`${species} · attempt ${captureAttemptId.slice(0, 8)}`}
        rightText={`体力 -20 · 余 ${currentStamina}`}
        rightTone="pink"
      />

      <SafetyStopBanner
        showStopFirst
        messages={outdoor.allowed ? [] : outdoor.messages}
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
        aria-disabled={!outdoor.allowed}
        style={outdoor.allowed ? undefined : { opacity: 0.55, pointerEvents: 'none' }}
      >
        <AnimalIcon species={session.species} size={120} />
        <CaptureProbabilityBar
          title="捕获判定"
          successRate={captureRate}
          bestMin={BEST_MIN}
          bestMax={BEST_MAX}
        />
        <div style={{ fontSize: 12, marginTop: 8, opacity: 0.75 }}>
          框 [{detection.boundingBox.map((n) => n.toFixed(2)).join(', ')}] ·{' '}
          {Math.round(detection.confidence * 100)}%
        </div>
      </div>
    </div>
  )
}
