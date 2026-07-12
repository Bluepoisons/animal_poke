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
import { runCaptureGeneration, type GeneratedAnimal } from '../../../services/capturePipeline'
import { enqueueGeneratedAnimal, flushSyncQueue } from '../../../services/syncQueue'
import { AnimalRepository } from '../../../db/repositories/animal-repository'
import { SyncQueueRepository } from '../../../db/repositories/sync-queue-repository'
import { generatedAnimalToRecord } from '../../../db/animal-record-mapper'
import {
  createPostHitTask,
  isRevealAllowed,
  loadPostHitTask,
  savePostHitTask,
  stageLabel,
  type PostHitTask,
} from '../capturePostHit'
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

const TINY_JPEG =
  'data:image/jpeg;base64,/9j/4AAQSkZJRgABAQAAAQABAAD/2wBDAAgGBgcGBQgHBwcJCQgKDBQNDAsLDBkSEw8UHRofHh0aHBwgJC4nICIsIxwcKDcpLDAxNDQ0Hyc5PTgyPC4zNDL/2wBDAQkJCQwLDBgNDRgyIRwhMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjL/wAARCAABAAEDASIAAhEBAxEB/8QAFQABAQAAAAAAAAAAAAAAAAAAAAn/xAAUEAEAAAAAAAAAAAAAAAAAAAAA/8QAFQEBAQAAAAAAAAAAAAAAAAAAAAX/xAAUEQEAAAAAAAAAAAAAAAAAAAAA/9oADAMBAAIQAxAAAAGcP//EABQQAQAAAAAAAAAAAAAAAAAAAAD/2gAIAQEAAQUCf//EABQRAQAAAAAAAAAAAAAAAAAAAAD/2gAIAQMBAT8Bf//EABQRAQAAAAAAAAAAAAAAAAAAAAD/2gAIAQIBAT8Bf//Z'

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
    bestMin: BEST_MIN,
    bestMax: BEST_MAX,
    label: '标准',
  }
  const lbs = useLbs()
  const weather = useWeather()
  const [enc, setEnc] = useState<EncounterState>(() => createEncounter(species, 3))
  // Keep the latest encounter available to event handlers without relying on a
  // state updater callback. State updater callbacks must stay pure: settling a
  // throw consumes stamina and starts async work, both of which are side effects.
  const encRef = useRef(enc)
  encRef.current = enc
  const [battery, setBattery] = useState<{ level: number | null; charging: boolean | null }>({
    level: null,
    charging: null,
  })
  const [postHit, setPostHit] = useState<PostHitTask>(
    () => loadPostHitTask(captureAttemptId) ?? createPostHitTask(captureAttemptId, species),
  )
  const chargingRef = useRef(false)
  const rafRef = useRef<number | null>(null)
  const powerRef = useRef(0)
  const dirRef = useRef(1)
  const pipelineRunning = useRef(false)
  const generatedCache = useRef<GeneratedAnimal | null>(null)

  const patchPostHit = useCallback((partial: Partial<PostHitTask>) => {
    setPostHit((prev) => {
      const next = savePostHitTask({ ...prev, ...partial, attemptId: prev.attemptId, species: prev.species })
      return next
    })
  }, [])

  useEffect(() => {
    if (!detectInferenceId || !detection) onInvalidAccess?.()
  }, [detectInferenceId, detection, onInvalidAccess])

  useEffect(() => {
    // reload persisted task when attempt id changes
    const loaded = loadPostHitTask(captureAttemptId)
    setPostHit(loaded ?? createPostHitTask(captureAttemptId, species))
    generatedCache.current = null
    pipelineRunning.current = false
  }, [captureAttemptId, species])

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
  const captureRate = useMemo(
    () => successProbability(power, profile.bestMin, profile.bestMax),
    [power, profile.bestMin, profile.bestMax],
  )

  const syncGeneratedAnimal = useCallback(
    async (generated: GeneratedAnimal) => {
      const position = lbs.state.playerLocation
      const queued = await enqueueGeneratedAnimal(generated, {
        lat: position?.lat,
        lng: position?.lng,
      })
      await flushSyncQueue()
      const persisted = await SyncQueueRepository.getById(queued.id)
      if (persisted?.status !== 'synced') {
        throw new Error(persisted?.lastError || 'sync_pending')
      }
    },
    [lbs.state.playerLocation],
  )

  const runPipeline = useCallback(
    async (task: PostHitTask) => {
      if (pipelineRunning.current) return
      if (task.stage === 'completed' || task.stage === 'failed') return
      if (task.stage === 'idle') return
      pipelineRunning.current = true
      try {
        let working = task
        let generated = generatedCache.current

        // analyze + value (runCaptureGeneration is multi-stage; mark analyzing first)
        if (!working.generated || !generated) {
          patchPostHit({ stage: 'analyzing', errorCode: null, errorMessage: null })
          let photoDataUrl = ''
          if (photoBlob) {
            photoDataUrl = await new Promise<string>((resolve, reject) => {
              const fr = new FileReader()
              fr.onload = () => resolve(String(fr.result || ''))
              fr.onerror = () => reject(fr.error || new Error('read_failed'))
              fr.readAsDataURL(photoBlob)
            })
          } else {
            photoDataUrl = TINY_JPEG
          }
          patchPostHit({ stage: 'generating' })
          generated = await runCaptureGeneration({
            sessionId: captureAttemptId,
            species,
            photoDataUrl,
            detectInferenceId,
            targetId: targetId || detection.targetId,
            boundingBox: detection.boundingBox,
          })
          generatedCache.current = generated
          working = savePostHitTask({
            ...working,
            stage: 'saving',
            generated: true,
            sessionId: generated.sessionId,
            valueInferenceId: generated.valueInferenceId || generated.inferenceRequestId,
          })
          setPostHit(working)
        }

        // local save — reveal only after this succeeds
        if (!working.saved && generated) {
          patchPostHit({ stage: 'saving' })
          const existing = await AnimalRepository.getById(generated.sessionId).catch(() => undefined)
          if (!existing) {
            const position = lbs.state.playerLocation
            await AnimalRepository.add(
              generatedAnimalToRecord(generated, {
                location: lbs.state.cityName,
                latitude: position?.lat,
                longitude: position?.lng,
              }),
            )
          }
          working = savePostHitTask({ ...working, stage: 'syncing', saved: true })
          setPostHit(working)
          // collection value toast only once after save
          const value = registerCapture(species)
          onToast(value.message)
          const rare = announceRareReveal(value.isFirst ? 'legendary' : 'common')
          if (rare) onToast(rare)
        }

        // sync — failure leaves local save intact
        if (working.saved && !working.synced && generated) {
          patchPostHit({ stage: 'syncing' })
          try {
            await syncGeneratedAnimal(generated)
            working = savePostHitTask({
              ...working,
              stage: 'completed',
              synced: true,
            })
            setPostHit(working)
            onToast('捕获成功')
            onSettled?.(true)
          } catch (syncErr) {
            working = savePostHitTask({
              ...working,
              stage: 'saved_pending_sync',
              synced: false,
              errorCode: 'sync_failed',
              errorMessage: syncErr instanceof Error ? syncErr.message : 'sync_failed',
            })
            setPostHit(working)
            onToast('已本地保存、待同步')
            onSettled?.(true)
          }
          return
        }

        if (working.stage === 'saved_pending_sync' && generated && !working.synced) {
          try {
            await syncGeneratedAnimal(generated)
            working = savePostHitTask({ ...working, stage: 'completed', synced: true, errorCode: null })
            setPostHit(working)
            onToast('同步完成')
          } catch {
            /* stay pending */
          }
        }
      } catch (err) {
        const msg = err instanceof Error ? err.message : 'pipeline_failed'
        const failed = savePostHitTask({
          ...task,
          stage: 'failed',
          errorCode: 'pipeline_failed',
          errorMessage: msg,
        })
        setPostHit(failed)
        onToast(`生成失败：${msg}`)
        onSettled?.(false)
      } finally {
        pipelineRunning.current = false
      }
    },
    [
      captureAttemptId,
      detectInferenceId,
      detection,
      lbs.state.cityName,
      lbs.state.playerLocation,
      onSettled,
      onToast,
      patchPostHit,
      photoBlob,
      species,
      syncGeneratedAnimal,
      targetId,
    ],
  )

  // Resume unfinished pipeline after refresh / remount (effect — never during render)
  useEffect(() => {
    if (
      postHit.stage === 'hit' ||
      postHit.stage === 'analyzing' ||
      postHit.stage === 'generating' ||
      postHit.stage === 'saving' ||
      postHit.stage === 'syncing' ||
      postHit.stage === 'saved_pending_sync'
    ) {
      void runPipeline(postHit)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- only re-enter when stage/attempt changes
  }, [postHit.stage, postHit.attemptId])

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
    const next = beginCharge(encRef.current)
    encRef.current = next
    setEnc(next)
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
      const nextEncounter = updatePower(encRef.current, next)
      encRef.current = nextEncounter
      setEnc(nextEncounter)
      rafRef.current = requestAnimationFrame(tick)
    }
    rafRef.current = requestAnimationFrame(tick)
  }, [profile.chargeSpeed])

  const throwNow = useCallback(() => {
    const currentEncounter = encRef.current
    const activeAttempt = currentAttempt(currentEncounter)
    if (!chargingRef.current && activeAttempt?.phase !== 'charging') return
    if (postHit.stage !== 'idle' && postHit.stage !== 'failed') return
    stopChargeLoop()

    const forceOk = isForceCaptureSuccess()
    if (forceOk) {
      powerRef.current = Math.round((profile.bestMin + profile.bestMax) / 2)
    }

    const powered = markThrown(updatePower(currentEncounter, powerRef.current))
    const result = settleAttempt(powered, {
      online: typeof navigator === 'undefined' ? true : navigator.onLine,
      stamina: currentStamina,
      consumeStamina,
      rng: forceOk ? () => 0 : undefined,
    })

    encRef.current = result.enc
    setEnc(result.enc)

    if (result.ok) {
      onToast(`力度 ${Math.round(powerRef.current)}`)
      const task = savePostHitTask({
        ...createPostHitTask(captureAttemptId, species),
        stage: 'hit',
        staminaConsumed: true,
      })
      setPostHit(task)
      void runPipeline(task)
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
      const left = result.enc.maxAttempts - result.enc.attempts.length
      onToast(`未命中 · 还可尝试 ${left} 次`)
    }
  }, [
    captureAttemptId,
    consumeStamina,
    currentStamina,
    onSettled,
    onToast,
    postHit.stage,
    profile.bestMax,
    profile.bestMin,
    runPipeline,
    species,
  ])

  const onPointerDown = (e: React.PointerEvent) => {
    if (!outdoor.allowed) return
    if (enc.locked || enc.success) return
    if (att?.settled) return
    if (postHit.stage !== 'idle' && postHit.stage !== 'failed') return
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
    if (postHit.stage !== 'idle' && postHit.stage !== 'failed') return
    const next = startNextAttempt(encRef.current)
    encRef.current = next
    setEnc(next)
    setPostHit(createPostHitTask(captureAttemptId, species))
    onToast('新的一次投掷机会')
  }

  const handleRetryPipeline = () => {
    if (postHit.stage !== 'failed' && postHit.stage !== 'saved_pending_sync') return
    const resume =
      postHit.stage === 'saved_pending_sync'
        ? postHit
        : savePostHitTask({
            ...postHit,
            stage: postHit.saved ? 'syncing' : postHit.generated ? 'saving' : 'hit',
            errorCode: null,
            errorMessage: null,
          })
    setPostHit(resume)
    void runPipeline(resume)
  }

  const reveal = isRevealAllowed(postHit.stage)
  const stageText = stageLabel(postHit.stage)

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
        className="ap-capture-stage"
        data-testid="capture-stage"
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
          bestMin={profile.bestMin}
          bestMax={profile.bestMax}
        />
        <div style={{ fontSize: 12, marginTop: 8, opacity: 0.75 }}>
          置信度 {Math.round(detection.confidence * 100)}% · 按住蓄力 / 空格键
        </div>
        {stageText ? (
          <div
            data-testid="capture-post-hit-stage"
            data-stage={postHit.stage}
            style={{ marginTop: 12, fontWeight: 600, color: 'var(--orange-dark, #E67300)' }}
          >
            {stageText}
          </div>
        ) : null}
        {reveal ? (
          <div
            data-testid="capture-success-reveal"
            style={{ marginTop: 8, fontWeight: 700, color: 'var(--orange-dark, #E67300)' }}
          >
            {postHit.stage === 'saved_pending_sync' ? '已本地保存、待同步' : '捕获成功'}
          </div>
        ) : null}
        {att?.phase === 'settled_fail' && canRetry(enc) && (
          <button type="button" className="ap-map-chip" style={{ marginTop: 12 }} onClick={handleRetry}>
            再试一次（新 attempt）
          </button>
        )}
        {(postHit.stage === 'failed' || postHit.stage === 'saved_pending_sync') && (
          <button
            type="button"
            className="ap-map-chip"
            style={{ marginTop: 12 }}
            data-testid="capture-retry-pipeline"
            onClick={handleRetryPipeline}
          >
            {postHit.stage === 'saved_pending_sync' ? '重试同步' : '重试生成'}
          </button>
        )}
      </div>
      <WelfareNotice />
    </div>
  )
}
