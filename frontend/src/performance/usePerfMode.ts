import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { JANK_SAMPLE_SIZE, PERF_STORAGE_KEY } from './constants'
import { decidePerfMode, updateJankRatio } from './logic'
import type { BatterySignals, NetworkSignals, PerfDecision, PerfSignals } from './types'

function readNetwork(): NetworkSignals {
  const online = typeof navigator === 'undefined' ? true : navigator.onLine
  const conn = (navigator as Navigator & {
    connection?: {
      effectiveType?: string
      saveData?: boolean
      downlink?: number
    }
  }).connection
  return {
    online,
    effectiveType: conn?.effectiveType ?? null,
    saveData: conn?.saveData ?? null,
    downlinkMbps: conn?.downlink ?? null,
  }
}

/**
 * Adaptive performance mode: continuous vs manual scan, upload budgets.
 * Pauses camera responsibility is exposed via decision.pauseCameraOnBackground.
 */
export function usePerfMode(opts?: { userDataSaver?: boolean }) {
  const [network, setNetwork] = useState<NetworkSignals>(() => readNetwork())
  const [battery, setBattery] = useState<BatterySignals>({ level: null, charging: null })
  const [jankRatio, setJankRatio] = useState(0)
  const jankSamples = useRef<number[]>([])
  const [hidden, setHidden] = useState(
    typeof document !== 'undefined' ? document.visibilityState === 'hidden' : false,
  )

  useEffect(() => {
    const onOnline = () => setNetwork(readNetwork())
    window.addEventListener('online', onOnline)
    window.addEventListener('offline', onOnline)
    const conn = (navigator as Navigator & { connection?: EventTarget }).connection
    conn?.addEventListener?.('change', onOnline)
    return () => {
      window.removeEventListener('online', onOnline)
      window.removeEventListener('offline', onOnline)
      conn?.removeEventListener?.('change', onOnline)
    }
  }, [])

  useEffect(() => {
    let cancelled = false
    const nav = navigator as Navigator & {
      getBattery?: () => Promise<{
        level: number
        charging: boolean
        addEventListener: (t: string, fn: () => void) => void
        removeEventListener: (t: string, fn: () => void) => void
      }>
    }
    if (typeof nav.getBattery !== 'function') return
    nav.getBattery().then((b) => {
      if (cancelled) return
      const sync = () => setBattery({ level: b.level, charging: b.charging })
      sync()
      b.addEventListener('levelchange', sync)
      b.addEventListener('chargingchange', sync)
    }).catch(() => {})
    return () => {
      cancelled = true
    }
  }, [])

  useEffect(() => {
    const onVis = () => setHidden(document.visibilityState === 'hidden')
    document.addEventListener('visibilitychange', onVis)
    return () => document.removeEventListener('visibilitychange', onVis)
  }, [])

  // Lightweight jank probe via rAF when page visible
  useEffect(() => {
    if (hidden) return
    let raf = 0
    let last = performance.now()
    const tick = (now: number) => {
      const dt = now - last
      last = now
      const { samples, ratio } = updateJankRatio(jankSamples.current, dt, JANK_SAMPLE_SIZE)
      jankSamples.current = samples
      setJankRatio(ratio)
      raf = requestAnimationFrame(tick)
    }
    raf = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(raf)
  }, [hidden])

  const signals: PerfSignals = useMemo(
    () => ({
      network,
      battery,
      jankRatio,
      userDataSaver: opts?.userDataSaver,
    }),
    [network, battery, jankRatio, opts?.userDataSaver],
  )

  const decision: PerfDecision = useMemo(() => decidePerfMode(signals), [signals])

  useEffect(() => {
    try {
      localStorage.setItem(
        PERF_STORAGE_KEY,
        JSON.stringify({ tier: decision.tier, scanMode: decision.scanMode, at: Date.now() }),
      )
    } catch { /* ignore */ }
  }, [decision.tier, decision.scanMode])

  const recordFrame = useCallback((frameMs: number) => {
    const { samples, ratio } = updateJankRatio(jankSamples.current, frameMs, JANK_SAMPLE_SIZE)
    jankSamples.current = samples
    setJankRatio(ratio)
  }, [])

  return {
    decision,
    signals,
    pageHidden: hidden,
    shouldPauseCamera: hidden && decision.pauseCameraOnBackground,
    recordFrame,
  }
}
