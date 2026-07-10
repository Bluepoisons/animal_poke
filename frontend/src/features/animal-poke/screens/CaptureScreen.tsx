import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import PageTitle from '../components/PageTitle'
import AnimalIcon from '../components/AnimalIcon'
import CaptureProbabilityBar from '../components/CaptureProbabilityBar'
import { useStamina } from '../../../stamina/useStamina'
import type { DetectionResult } from '../../../services/visionDetect'
import type { SpeciesType } from '../../../types'
import {
  BEST_MAX,
  BEST_MIN,
  SPECIES_THROW_PROFILES,
  beginCharge,
  canRetry,
  createEncounter,
  currentAttempt,
  markThrown,
  settleAttempt,
  startNextAttempt,
  successProbability,
  updatePower,
  type EncounterState,
} from '../captureAttempt'

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
  captureAttemptId,
  onSettled,
  onInvalidAccess,
}: CaptureScreenProps) {
  const { state: staminaState, consumeStamina } = useStamina()
  const currentStamina = staminaState.currentStamina
  const profile = SPECIES_THROW_PROFILES[species]
  const [enc, setEnc] = useState<EncounterState>(() => createEncounter(species, 3))
  const chargingRef = useRef(false)
  const rafRef = useRef<number | null>(null)
  const powerRef = useRef(0)
  const dirRef = useRef(1)
  const settledOnce = useRef(false)

  useEffect(() => {
    if (!detectInferenceId || !detection) onInvalidAccess?.()
  }, [detectInferenceId, detection, onInvalidAccess])

  useEffect(() => {
    return () => {
      if (rafRef.current) cancelAnimationFrame(rafRef.current)
    }
  }, [])

  const att = currentAttempt(enc)
  const power = att?.power ?? 0
  const captureRate = useMemo(() => successProbability(power), [power])

  const stopChargeLoop = () => {
    chargingRef.current = false
    if (rafRef.current) {
      cancelAnimationFrame(rafRef.current)
      rafRef.current = null
    }
  }

  const startChargeLoop = useCallback(() => {
    stopChargeLoop()
    chargingRef.current = true
    powerRef.current = 0
    dirRef.current = 1
    setEnc((e) => beginCharge(e))
    const tick = () => {
      if (!chargingRef.current) return
      const speed = 1.8 * profile.chargeSpeed
      let next = powerRef.current + dirRef.current * speed
      if (next >= 100) {
        next = 100
        dirRef.current = -1
      } else if (next <= 0) {
        next = 0
        dirRef.current = 1
      }
      powerRef.current = next
      setEnc((e) => updatePower(e, next))
      rafRef.current = requestAnimationFrame(tick)
    }
    rafRef.current = requestAnimationFrame(tick)
  }, [profile.chargeSpeed])

  const throwNow = useCallback(() => {
    if (!chargingRef.current && att?.phase !== 'charging') return
    stopChargeLoop()
    setEnc((e) => {
      let next = markThrown(updatePower(e, powerRef.current))
      const result = settleAttempt(next, {
        online: typeof navigator === 'undefined' ? true : navigator.onLine,
        stamina: currentStamina,
        consumeStamina: (n) => consumeStamina(n),
      })
      next = result.enc
      if (result.ok) {
        onToast(`捕获成功：${species} · 力度 ${powerRef.current}`)
        if (!settledOnce.current) {
          settledOnce.current = true
          onSettled?.(true)
        }
      } else if (result.reason === 'no_stamina') {
        onToast('体力不足')
      } else if (result.reason === 'offline') {
        onToast('离线无法捕获')
      } else if (result.reason === 'max_attempts') {
        onToast('本轮机会已用尽')
        onSettled?.(false)
      } else if (result.reason === 'already_settled') {
        onToast('本轮已结算')
      } else {
        onToast(`未命中 · 还可尝试 ${next.maxAttempts - next.attempts.length} 次`)
      }
      return next
    })
  }, [att?.phase, currentStamina, consumeStamina, onToast, onSettled, species])

  const onPointerDown = (e: React.PointerEvent) => {
    if (enc.locked || enc.success) return
    if (att?.settled) return
    e.currentTarget.setPointerCapture?.(e.pointerId)
    startChargeLoop()
  }

  const onPointerUp = () => {
    if (chargingRef.current) throwNow()
  }

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === ' ' || e.key === 'Enter') {
      e.preventDefault()
      if (!chargingRef.current && att && !att.settled && !enc.locked) {
        startChargeLoop()
      } else if (chargingRef.current) {
        throwNow()
      }
    }
  }

  const handleRetry = () => {
    if (!canRetry(enc)) return
    settledOnce.current = false
    setEnc((e) => startNextAttempt(e))
    onToast('新的一次投掷机会')
  }

  return (
    <div className="ap-screen">
      <PageTitle
        title="CAPTURE"
        subtitle={`${species} · ${profile.label} · attempt ${(att?.index ?? 0) + 1}/${enc.maxAttempts}`}
        rightText={`体力 -20 · 余 ${currentStamina}`}
        rightTone="pink"
      />

      <div
        className="ap-capture-stage"
        onPointerDown={onPointerDown}
        onPointerUp={onPointerUp}
        onPointerCancel={stopChargeLoop}
        onPointerLeave={() => {
          /* 不自动 throw，避免误触；松开按钮时 throw */
        }}
        onKeyDown={onKeyDown}
        role="button"
        tabIndex={0}
        aria-label="按住蓄力，松开投掷"
      >
        <AnimalIcon species={species} size={120} />
        <CaptureProbabilityBar
          title={att?.phase === 'charging' ? `蓄力 ${power}` : '捕获判定'}
          successRate={captureRate}
          bestMin={profile.bestMin}
          bestMax={profile.bestMax}
        />
        <div style={{ fontSize: 12, marginTop: 8, opacity: 0.75 }}>
          置信度 {Math.round(detection.confidence * 100)}% · 按住蓄力 / 空格键
        </div>
        {att?.phase === 'settled_fail' && canRetry(enc) && (
          <button type="button" className="ap-map-chip" style={{ marginTop: 12 }} onClick={handleRetry}>
            再试一次（新 attempt）
          </button>
        )}
        {enc.success && (
          <div style={{ marginTop: 12, fontWeight: 700, color: 'var(--orange-dark, #E67300)' }}>
            捕获成功
          </div>
        )}
      </div>
    </div>
  )
}
