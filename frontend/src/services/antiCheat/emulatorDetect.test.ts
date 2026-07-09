import { describe, it, expect, vi, beforeEach } from 'vitest'
import { detectEmulator } from './emulatorDetect'

describe('emulatorDetect', () => {
  beforeEach(() => {
    vi.stubGlobal('navigator', {
      userAgent: 'Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15',
      hardwareConcurrency: 6,
      deviceMemory: 4,
      maxTouchPoints: 5,
    })
    vi.stubGlobal('window', {
      screen: { width: 390, height: 844 },
    })
    vi.stubGlobal('DeviceMotionEvent', function DeviceMotionEvent() {})
  })

  it('should return low risk for real device signals', () => {
    const result = detectEmulator()
    expect(result.riskScore).toBeLessThan(50)
    expect(result.isEmulator).toBe(false)
  })

  it('should detect emulator UA', () => {
    vi.stubGlobal('navigator', {
      userAgent: 'Mozilla/5.0 (Linux; Android 9; sdk_gm x86) — Android SDK built for x86',
      hardwareConcurrency: 6,
      deviceMemory: 4,
      maxTouchPoints: 5,
    })
    const result = detectEmulator()
    expect(result.riskScore).toBeGreaterThanOrEqual(30)
  })

  it('should detect low CPU cores', () => {
    vi.stubGlobal('navigator', {
      userAgent: 'Mozilla/5.0 (Linux; Android 10; Pixel 4)',
      hardwareConcurrency: 1,
      deviceMemory: 4,
      maxTouchPoints: 5,
    })
    const result = detectEmulator()
    const cpuSignal = result.signals.find(s => s.name === 'cpu_cores')
    expect(cpuSignal?.suspicious).toBe(true)
  })

  it('should detect missing sensors on mobile UA', () => {
    // 重新设置：移动端 UA + 合理硬件 + 不 stub DeviceMotionEvent（使其 undefined）
    vi.stubGlobal('navigator', {
      userAgent: 'Mozilla/5.0 (Linux; Android 10; Pixel 4)',
      hardwareConcurrency: 8,
      deviceMemory: 8,
      maxTouchPoints: 5,
    })
    vi.stubGlobal('window', {
      screen: { width: 390, height: 844 },
    })
    // 不 stub DeviceMotionEvent → typeof 检测为 undefined → suspicious
    vi.stubGlobal('DeviceMotionEvent', undefined)
    const result = detectEmulator()
    const sensorSignal = result.signals.find(s => s.name === 'sensor_available')
    expect(sensorSignal?.suspicious).toBe(true)
  })

  it('should cap riskScore at 100', () => {
    vi.stubGlobal('navigator', {
      userAgent: 'sdk_gm emulator genymotion',
      hardwareConcurrency: 1,
      deviceMemory: 1,
      maxTouchPoints: 0,
    })
    vi.stubGlobal('window', {
      screen: { width: 320, height: 480 },
    })
    const result = detectEmulator()
    expect(result.riskScore).toBeLessThanOrEqual(100)
  })
})
