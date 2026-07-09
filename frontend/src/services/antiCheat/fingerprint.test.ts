import { describe, it, expect, vi, beforeEach } from 'vitest'
import { collectFingerprint } from './fingerprint'

describe('fingerprint', () => {
  beforeEach(() => {
    vi.stubGlobal('navigator', {
      userAgent: 'Mozilla/5.0 (iPhone; CPU iPhone OS 16_0)',
      hardwareConcurrency: 6,
      deviceMemory: 4,
      maxTouchPoints: 5,
      language: 'zh-CN',
    })
    vi.stubGlobal('window', {
      screen: { width: 390, height: 844, colorDepth: 30 },
    })
    vi.stubGlobal('Intl', {
      DateTimeFormat: () => ({
        resolvedOptions: () => ({ timeZone: 'Asia/Shanghai' }),
      }),
    })
  })

  it('should produce a non-empty fingerprint hash', () => {
    const fp = collectFingerprint()
    expect(fp.fingerprint).toBeTruthy()
    expect(fp.fingerprint.length).toBeGreaterThan(0)
  })

  it('should produce consistent hash for same inputs', () => {
    const fp1 = collectFingerprint()
    const fp2 = collectFingerprint()
    expect(fp1.fingerprint).toBe(fp2.fingerprint)
  })

  it('should include all component hashes', () => {
    const fp = collectFingerprint()
    expect(fp.uaHash).toBeTruthy()
    expect(fp.screenHash).toBeTruthy()
    expect(fp.gpuHash).toBeTruthy()
    expect(fp.localeHash).toBeTruthy()
    expect(fp.hardwareHash).toBeTruthy()
  })

  it('should produce different hash for different UA', () => {
    const fp1 = collectFingerprint()
    vi.stubGlobal('navigator', {
      userAgent: 'Mozilla/5.0 (Linux; Android 12; Pixel 6)',
      hardwareConcurrency: 6,
      deviceMemory: 4,
      maxTouchPoints: 5,
      language: 'zh-CN',
    })
    const fp2 = collectFingerprint()
    expect(fp1.fingerprint).not.toBe(fp2.fingerprint)
  })
})
