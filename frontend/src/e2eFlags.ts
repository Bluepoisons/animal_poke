/**
 * E2E / hard-gate 窗口标志（Playwright init script 写入）。
 * 生产路径不设置这些字段。
 */
export function isForceCameraReady(): boolean {
  return typeof window !== 'undefined' && window.__AP_FORCE_CAMERA_READY === true
}

export function isForceCaptureSuccess(): boolean {
  return typeof window !== 'undefined' && window.__AP_FORCE_CAPTURE_SUCCESS === true
}

declare global {
  interface Window {
    __AP_FORCE_CAMERA_READY?: boolean
    __AP_FORCE_CAPTURE_SUCCESS?: boolean
  }
}

export {}
