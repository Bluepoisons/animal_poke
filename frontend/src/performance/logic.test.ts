import { describe, it, expect } from 'vitest'
import {
  decidePerfMode,
  isSlowNetwork,
  isLowBattery,
  updateJankRatio,
  virtualWindow,
  pickThumbnailSrc,
} from './logic'
import type { PerfSignals } from './types'

const base: PerfSignals = {
  network: { online: true, effectiveType: '4g', saveData: false, downlinkMbps: 10 },
  battery: { level: 0.9, charging: false },
  jankRatio: 0,
  userDataSaver: false,
}

describe('decidePerfMode', () => {
  it('full continuous when healthy', () => {
    const d = decidePerfMode(base)
    expect(d.tier).toBe('full')
    expect(d.scanMode).toBe('continuous')
    expect(d.uploadMaxEdge).toBe(1280)
  })

  it('manual + saver on Save-Data / slow net', () => {
    const d = decidePerfMode({
      ...base,
      network: { ...base.network, saveData: true, effectiveType: '2g' },
    })
    expect(d.scanMode).toBe('manual')
    expect(['saver', 'low']).toContain(d.tier)
    expect(d.reasons).toContain('data_saver')
  })

  it('low tier on low battery', () => {
    const d = decidePerfMode({
      ...base,
      battery: { level: 0.1, charging: false },
    })
    expect(d.tier).toBe('low')
    expect(d.scanMode).toBe('manual')
    expect(d.uploadMaxEdge).toBe(640)
  })

  it('low tier on jank', () => {
    const d = decidePerfMode({ ...base, jankRatio: 0.5 })
    expect(d.tier).toBe('low')
    expect(d.reasons).toContain('jank')
  })

  it('user data saver forces saver', () => {
    const d = decidePerfMode({ ...base, userDataSaver: true })
    expect(d.reasons).toContain('data_saver')
    expect(d.scanMode).toBe('manual')
  })
})

describe('helpers', () => {
  it('isSlowNetwork', () => {
    expect(isSlowNetwork({ online: true, effectiveType: '4g' })).toBe(false)
    expect(isSlowNetwork({ online: true, effectiveType: '2g' })).toBe(true)
    expect(isSlowNetwork({ online: false })).toBe(true)
  })

  it('isLowBattery ignores charging', () => {
    expect(isLowBattery({ level: 0.1, charging: true })).toBe(false)
    expect(isLowBattery({ level: 0.1, charging: false })).toBe(true)
  })

  it('updateJankRatio rolling', () => {
    let samples: number[] = []
    let ratio = 0
    for (let i = 0; i < 10; i++) {
      const r = updateJankRatio(samples, i % 2 === 0 ? 50 : 10, 10)
      samples = r.samples
      ratio = r.ratio
    }
    expect(ratio).toBeCloseTo(0.5, 5)
  })

  it('virtualWindow', () => {
    const w = virtualWindow(100, 500, 400, 50, 2)
    expect(w.start).toBeGreaterThanOrEqual(0)
    expect(w.end).toBeGreaterThan(w.start)
    expect(w.offsetY).toBe(w.start * 50)
  })

  it('pickThumbnailSrc', () => {
    expect(pickThumbnailSrc('full.jpg', 't.jpg', true)).toBe('t.jpg')
    expect(pickThumbnailSrc('full.jpg', undefined, true)).toBe('full.jpg')
  })
})
