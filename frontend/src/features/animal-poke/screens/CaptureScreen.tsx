import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import PageTitle from '../components/PageTitle'
import AnimalIcon from '../components/AnimalIcon'
import CaptureProbabilityBar from '../components/CaptureProbabilityBar'
import { useStamina } from '../../../stamina/useStamina'
import { useLbs } from '../../../lbs/useLbs'
import { useWeather } from '../../../weather/useWeather'
import SafetyStopBanner from '../../../outdoorSafety/SafetyStopBanner'
import { evaluateOutdoorSafety } from '../../../outdoorSafety/logic'
import type { DetectionResult } from '../../../services/visionDetect'
import type { SpeciesType } from '../../../types'
import WelfareNotice from '../components/WelfareNotice'
import { announceRareReveal } from '../feedbackPrefs'
import { registerCapture } from '../collectionValue'
import { isForceCaptureSuccess } from '../../../e2eFlags'
import { runCaptureGeneration } from '../../../services/capturePipeline'
import { enqueueGeneratedAnimal, flushSyncQueue } from '../../../services/syncQueue'
import { AnimalRepository } from '../../../db/repositories/animal-repository'
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
  /** retained for pipeline handoff; throw UX currently does not re-upload */
  photoBlob?: Blob | null
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
  photoBlob,
  targetId,
  captureAttemptId,
  onSettled,
  onInvalidAccess,
}: CaptureScreenProps) {
  const { state: staminaState, consumeStamina } = useStamina()
  const currentStamina = staminaState.currentStamina
  const profile = SPECIES_THROW_PROFILES[species] ?? {
    species,
    chargeSpeed: 1,
    bestMin: 40,
    bestMax: 80,
    label: '标准',
  }
  const lbs = useLbs()
  const weather = useWeather()
  const [enc, setEnc] = useState<EncounterState>(() => createEncounter(species, 3))
  const [battery, setBattery] = useState<{ level: number | null; charging: boolean | null }>({
    level: null,
    charging: null,
  })
  const chargingRef = useRef(false)
  const rafRef = useRef<number | null>(null)
  const powerRef = useRef(0)
  const dirRef = useRef(1)
  const settledOnce = useRef(false)

  useEffect(() => {
    if (!detectInferenceId || !detection) onInvalidAccess?.()
  }, [detectInferenceId, detection, onInvalidAccess])

  useEffect(() => {
    let cancelled = false
    const nav = navigator as Navigator & {
      getBattery?: () => Promise<{ level: number; charging: boolean }>
    }
    if (typeof nav.getBattery === 'function') {
      nav
        .getBattery()
        .then((b) => {
          if (!cancelled) setBattery({ level: b.level, charging: b.charging })
        })
        .catch(() => {})
    }
    return () => {
      cancelled = true
      if (rafRef.current) cancelAnimationFrame(rafRef.current)
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
      const forceOk = isForceCaptureSuccess()
      if (forceOk) {
        // 命中最佳区间中点，保证 successProbability 足够高
        powerRef.current = Math.round((profile.bestMin + profile.bestMax) / 2)
      }
      let next = markThrown(updatePower(e, powerRef.current))
      const result = settleAttempt(next, {
        online: typeof navigator === 'undefined' ? true : navigator.onLine,
        stamina: currentStamina,
        consumeStamina: (n) => consumeStamina(n),
        rng: forceOk ? () => 0 : undefined,
      })
      next = result.enc
      if (result.ok) {
        const value = registerCapture(species)
        onToast(value.message)
        onToast(`力度 ${Math.round(powerRef.current)}`)
        const rare = announceRareReveal(value.isFirst ? 'legendary' : 'common')
        if (rare) onToast(rare)
        if (!settledOnce.current) {
          settledOnce.current = true
          // Fire-and-forget analyze→value→sync pipeline (AP-014 hard gate)
          void (async () => {
            try {
              let photoDataUrl = ''
              if (photoBlob) {
                photoDataUrl = await new Promise<string>((resolve, reject) => {
                  const fr = new FileReader()
                  fr.onload = () => resolve(String(fr.result || ''))
                  fr.onerror = () => reject(fr.error || new Error('read_failed'))
                  fr.readAsDataURL(photoBlob)
                })
              } else {
                // E2E/minimal path: tiny jpeg data URL so FormData still posts
                photoDataUrl =
                  'data:image/jpeg;base64,/9j/4AAQSkZJRgABAQAAAQABAAD/2wBDAAgGBgcGBQgHBwcJCQgKDBQNDAsLDBkSEw8UHRofHh0aHBwgJC4nICIsIxwcKDcpLDAxNDQ0Hyc5PTgyPC4zNDL/2wBDAQkJCQwLDBgNDRgyIRwhMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjL/wAARCAABAAEDASIAAhEBAxEB/8QAFQABAQAAAAAAAAAAAAAAAAAAAAn/xAAUEAEAAAAAAAAAAAAAAAAAAAAA/8QAFQEBAQAAAAAAAAAAAAAAAAAAAAX/xAAUEQEAAAAAAAAAAAAAAAAAAAAA/9oADAMBAAIQAxAAAAGcP//EABQQAQAAAAAAAAAAAAAAAAAAAAD/2gAIAQEAAQUCf//EABQRAQAAAAAAAAAAAAAAAAAAAAD/2gAIAQMBAT8Bf//EABQRAQAAAAAAAAAAAAAAAAAAAAD/2gAIAQIBAT8Bf//Z'
              }
              const generated = await runCaptureGeneration({
                sessionId: captureAttemptId,
                species,
                photoDataUrl,
                detectInferenceId,
                targetId: targetId || detection.targetId,
                boundingBox: detection.boundingBox,
              })
              await AnimalRepository.add({
                id: generated.sessionId,
                uuid: generated.sessionId,
                species: generated.species as SpeciesType,
                rarity: generated.value.rarity,
                breed: generated.analysis.breed,
                hp: generated.value.hp,
                atk: generated.value.atk,
                def: generated.value.def,
                spd: generated.value.spd,
                className: generated.value.class,
                element: generated.value.element,
                narrative: generated.value.narrative,
                fiction: generated.value.fiction ?? true,
                disclaimer: generated.value.disclaimer ?? '虚构花絮，非真实个体传记',
                layer: generated.value.layer ?? 'fictional_vignette',
                capturedAt: Date.now(),
                inferenceRequestId: generated.valueInferenceId || generated.inferenceRequestId,
                synced: false,
              } as never)
              await enqueueGeneratedAnimal(generated)
              await flushSyncQueue()
              onToast('捕获成功')
              onSettled?.(true)
            } catch (err) {
              const msg = err instanceof Error ? err.message : 'pipeline_failed'
              onToast(`生成失败：${msg}`)
              onSettled?.(false)
            }
          })()
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
  }, [att?.phase, currentStamina, consumeStamina, onToast, onSettled, species, profile.bestMin, profile.bestMax, photoBlob, detectInferenceId, targetId, detection, captureAttemptId])

  const onPointerDown = (e: React.PointerEvent) => {
    if (!outdoor.allowed) return
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
      if (!outdoor.allowed) {
        onToast(outdoor.messages[0] ?? '户外捕获已暂停，请先停下再操作')
        return
      }
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
    <div className="ap-screen" data-testid="capture-screen">
      <PageTitle
        title="CAPTURE"
        subtitle={`${species} · ${profile.label} · attempt ${(att?.index ?? 0) + 1}/${enc.maxAttempts}`}
        rightText={`体力 -20 · 余 ${currentStamina}`}
        rightTone="pink"
      />

      <SafetyStopBanner showStopFirst messages={outdoor.allowed ? [] : outdoor.messages} />

      <div
        className="ap-capture-stage" data-testid="capture-stage"
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
        aria-disabled={!outdoor.allowed}
        style={outdoor.allowed ? undefined : { opacity: 0.55, pointerEvents: 'none' }}
      >
        <AnimalIcon species={species} size={120} />
        <CaptureProbabilityBar
          title={att?.phase === 'charging' ? `蓄力 ${power}` : '捕获判定'}
          successRate={captureRate}
          bestMin={BEST_MIN}
          bestMax={BEST_MAX}
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
      <WelfareNotice />
    </div>
  )
}
