import { describe, it, expect } from 'vitest'
import { evaluateRisk } from './report'
import type { DeviceSecurityReport } from './types'

function mockReport(overrides: Partial<DeviceSecurityReport>): DeviceSecurityReport {
  return {
    emulatorCheck: { isEmulator: false, signals: [], riskScore: 0 },
    rootCheck: { isRooted: false, signals: [], riskScore: 0 },
    locationProof: { mockDetected: false },
    timeSync: { isManipulated: false },
    fingerprint: {
      uaHash: 'abc',
      screenHash: 'def',
      gpuHash: 'ghi',
      localeHash: 'jkl',
      hardwareHash: 'mno',
      fingerprint: 'pqr',
    },
    collectedAt: Date.now(),
    ...overrides,
  }
}

describe('report — evaluateRisk', () => {
  it('should return low risk for clean device', () => {
    const result = evaluateRisk(mockReport({}))
    expect(result.level).toBe('low')
    expect(result.score).toBe(0)
  })

  it('should return medium risk for partial emulator signals', () => {
    const result = evaluateRisk(mockReport({
      emulatorCheck: { isEmulator: false, signals: [], riskScore: 40 },
    }))
    expect(result.level).toBe('medium')
    expect(result.score).toBeGreaterThanOrEqual(30)
  })

  it('should return high risk for detected emulator', () => {
    const result = evaluateRisk(mockReport({
      emulatorCheck: { isEmulator: true, signals: [], riskScore: 55 },
    }))
    expect(result.level).toBe('high')
    expect(result.score).toBeGreaterThanOrEqual(50)
  })

  it('should return critical risk for emulator + root', () => {
    const result = evaluateRisk(mockReport({
      emulatorCheck: { isEmulator: true, signals: [], riskScore: 60 },
      rootCheck: { isRooted: true, signals: ['webdriver'], riskScore: 100 },
    }))
    expect(result.level).toBe('critical')
    expect(result.score).toBeGreaterThanOrEqual(80)
  })

  it('should add 20 for time manipulation', () => {
    const result = evaluateRisk(mockReport({
      timeSync: { isManipulated: true },
    }))
    expect(result.score).toBe(20)
    expect(result.level).toBe('low')
  })

  it('should add 30 for location mock detected', () => {
    const result = evaluateRisk(mockReport({
      locationProof: { mockDetected: true },
    }))
    expect(result.score).toBe(30)
    expect(result.level).toBe('medium')
  })

  it('should cap score at 100', () => {
    const result = evaluateRisk(mockReport({
      emulatorCheck: { isEmulator: true, signals: [], riskScore: 100 },
      rootCheck: { isRooted: true, signals: ['x'], riskScore: 100 },
      timeSync: { isManipulated: true },
      locationProof: { mockDetected: true },
    }))
    expect(result.score).toBe(100)
    expect(result.level).toBe('critical')
  })
})
