import { describe, it, expect, beforeEach } from 'vitest'
import { canStartScan, loadScanBudget, recordScanAttempt, saveScanBudget } from './scanBudget'

describe('scanBudget', () => {
  beforeEach(() => {
    localStorage.clear()
    sessionStorage.clear()
  })

  it('defaults to manual mode with daily quota', () => {
    const s = loadScanBudget()
    expect(s.mode).toBe('manual')
    expect(s.dailyQuota).toBe(100)
  })

  it('blocks when quota exhausted', () => {
    saveScanBudget({
      ...loadScanBudget(),
      usedToday: 100,
      dailyQuota: 100,
      dayKey: new Date().toISOString().slice(0, 10),
    })
    const g = canStartScan()
    expect(g.ok).toBe(false)
    if (!g.ok) expect(g.reason).toBe('quota_exhausted')
  })

  it('records attempts and enforces interval', () => {
    const a = recordScanAttempt()
    expect(a.usedToday).toBe(1)
    const g = canStartScan(a, a.lastScanAt + 100)
    expect(g.ok).toBe(false)
  })
})
