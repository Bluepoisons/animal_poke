import type { DeviceFingerprint } from './types'

/** 简单字符串哈希（djb2 算法，输出 16 进制） */
function simpleHash(input: string): string {
  let hash = 5381
  for (let i = 0; i < input.length; i++) {
    hash = ((hash << 5) + hash + input.charCodeAt(i)) & 0xffffffff
  }
  return (hash >>> 0).toString(16).padStart(8, '0')
}

/**
 * 采集设备指纹。
 * 综合多维特征生成指纹哈希，用于设备唯一性辅助识别。
 */
export function collectFingerprint(): DeviceFingerprint {
  // GPU renderer
  let renderer = 'unknown'
  try {
    const canvas = document.createElement('canvas')
    const gl = canvas.getContext('webgl')
    if (gl) {
      const dbg = gl.getExtension('WEBGL_debug_renderer_info')
      if (dbg) {
        renderer = String(gl.getParameter(dbg.UNMASKED_RENDERER_WEBGL))
      }
    }
  } catch { /* noop */ }

  const uaHash = simpleHash(navigator.userAgent)
  const screenHash = simpleHash(`${window.screen.width}x${window.screen.height}x${window.screen.colorDepth}`)
  const gpuHash = simpleHash(renderer)
  const localeHash = simpleHash(
    `${Intl.DateTimeFormat().resolvedOptions().timeZone}|${navigator.language}`,
  )
  const hardwareHash = simpleHash(
    `${navigator.hardwareConcurrency}|${(navigator as unknown as { deviceMemory?: number }).deviceMemory || 0}|${navigator.maxTouchPoints}`,
  )

  const fingerprint = simpleHash(uaHash + screenHash + gpuHash + localeHash + hardwareHash)

  return {
    uaHash,
    screenHash,
    gpuHash,
    localeHash,
    hardwareHash,
    fingerprint,
  }
}
