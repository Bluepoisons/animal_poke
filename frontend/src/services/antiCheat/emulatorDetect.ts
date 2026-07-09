import type { EmulatorCheckResult, EmulatorSignal } from './types'

/**
 * 模拟器检测 — 通过浏览器 API 采集多维信号。
 * Web 端检测可被绕过，但提高攻击门槛。
 */
export function detectEmulator(): EmulatorCheckResult {
  const signals: EmulatorSignal[] = []

  // 1. User-Agent 检测
  const ua = navigator.userAgent.toLowerCase()
  const uaSuspicious = /sdk|emulator|simulator|genymotion|bluestacks|nox|ldplayer/i.test(ua)
  signals.push({
    name: 'user_agent',
    value: navigator.userAgent,
    suspicious: uaSuspicious,
    weight: 30,
  })

  // 2. GPU 渲染器检测
  let renderer = 'unknown'
  try {
    const canvas = document.createElement('canvas')
    const gl = canvas.getContext('webgl') || canvas.getContext('webgl2')
    if (gl) {
      const dbg = gl.getExtension('WEBGL_debug_renderer_info')
      if (dbg) {
        renderer = String(gl.getParameter(dbg.UNMASKED_RENDERER_WEBGL))
      }
    }
  } catch { /* WebGL not available */ }
  const rendererSuspicious = /swiftshader|android emulator|google swiftshader|llvmpipe/i.test(renderer)
  signals.push({
    name: 'gpu_renderer',
    value: renderer,
    suspicious: rendererSuspicious,
    weight: 25,
  })

  // 3. 硬件并发数
  const cores = navigator.hardwareConcurrency || 0
  signals.push({
    name: 'cpu_cores',
    value: String(cores),
    suspicious: cores > 0 && cores <= 2,
    weight: 10,
  })

  // 4. 设备内存（Chrome 系，单位 GB）
  const memory = (navigator as unknown as { deviceMemory?: number }).deviceMemory || 0
  signals.push({
    name: 'device_memory',
    value: `${memory}GB`,
    suspicious: memory > 0 && memory <= 1,
    weight: 10,
  })

  // 5. 触摸点数
  const touchPoints = navigator.maxTouchPoints || 0
  const isMobileUA = /Mobile|Android|iPhone/i.test(ua)
  signals.push({
    name: 'touch_points',
    value: String(touchPoints),
    suspicious: touchPoints === 0 && isMobileUA,
    weight: 10,
  })

  // 6. 屏幕尺寸（模拟器常见低分辨率）
  const screenArea = window.screen.width * window.screen.height
  signals.push({
    name: 'screen_resolution',
    value: `${window.screen.width}x${window.screen.height}`,
    suspicious: screenArea <= 320 * 480,
    weight: 5,
  })

  // 7. 传感器存在性
  const sensorAvailable = typeof DeviceMotionEvent !== 'undefined'
  signals.push({
    name: 'sensor_available',
    value: sensorAvailable ? 'available' : 'missing',
    suspicious: !sensorAvailable && isMobileUA,
    weight: 10,
  })

  // 计算风险分
  const riskScore = Math.min(
    100,
    signals.filter(s => s.suspicious).reduce((sum, s) => sum + s.weight, 0),
  )

  return {
    isEmulator: riskScore >= 50,
    signals,
    riskScore,
  }
}
