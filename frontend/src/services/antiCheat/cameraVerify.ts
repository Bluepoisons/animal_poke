import type { CameraProof, MotionSample } from './types'

/**
 * 采集相机证明数据 — 验证为实时取景而非相册导入。
 */
export async function collectCameraProof(
  stream: MediaStream,
  sessionNonce: string,
): Promise<CameraProof> {
  const tracks = stream.getVideoTracks()
  if (tracks.length === 0) throw new Error('no video tracks')

  const track = tracks[0]
  const settings = track.getSettings()

  return {
    trackLabel: track.label,
    trackSettings: settings,
    trackState: track.readyState,
    frameTimestamps: [],
    sessionNonce,
  }
}

/**
 * 检测虚拟摄像头 — track.label 包含可疑关键词。
 */
export function isVirtualCamera(label: string): boolean {
  const suspicious = ['virtual', 'obs', 'dummy', 'screen', 'capture']
  const lower = label.toLowerCase()
  return suspicious.some(s => lower.includes(s))
}

/**
 * 启动运动传感器采集，返回停止函数。
 * 用于交叉验证手持设备的微小抖动，排除静态翻拍。
 */
export function startMotionSampling(
  samples: MotionSample[],
  intervalMs = 200,
): () => void {
  const handler = (event: DeviceMotionEvent) => {
    const accel = event.accelerationIncludingGravity
    const rot = event.rotationRate
    if (accel && rot) {
      samples.push({
        timestamp: performance.now(),
        accelX: accel.x ?? 0,
        accelY: accel.y ?? 0,
        accelZ: accel.z ?? 0,
        rotationAlpha: rot.alpha ?? 0,
        rotationBeta: rot.beta ?? 0,
        rotationGamma: rot.gamma ?? 0,
      })
    }
  }
  window.addEventListener('devicemotion', handler)
  return () => window.removeEventListener('devicemotion', handler)
}

/**
 * 分析传感器样本方差 — 方差 ≈ 0 说明可能是翻拍/静态照片。
 */
export function analyzeMotionVariance(samples: MotionSample[]): {
  variance: number
  isStatic: boolean
} {
  if (samples.length < 2) {
    return { variance: 0, isStatic: true }
  }

  const accelMag = samples.map(s =>
    Math.sqrt(s.accelX ** 2 + s.accelY ** 2 + s.accelZ ** 2),
  )
  const mean = accelMag.reduce((a, b) => a + b, 0) / accelMag.length
  const variance =
    accelMag.reduce((sum, v) => sum + (v - mean) ** 2, 0) / accelMag.length

  return {
    variance,
    isStatic: variance < 0.01,
  }
}
