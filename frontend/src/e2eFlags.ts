/**
 * E2E / hard-gate 窗口标志（Playwright init script 写入）。
 * 仅测试构建读取这些字段，正式构建会在编译期关闭并移除分支。
 */
const E2E_HOOKS_ENABLED =
  import.meta.env.MODE === 'test' || import.meta.env.VITE_ENABLE_E2E_HOOKS === '1'

export function isForceCameraReady(): boolean {
  return E2E_HOOKS_ENABLED && typeof window !== 'undefined' && window.__AP_FORCE_CAMERA_READY === true
}

export function isForceCaptureSuccess(): boolean {
  return E2E_HOOKS_ENABLED && typeof window !== 'undefined' && window.__AP_FORCE_CAPTURE_SUCCESS === true
}

declare global {
  interface Window {
    __AP_FORCE_CAMERA_READY?: boolean
    __AP_FORCE_CAPTURE_SUCCESS?: boolean
  }
}

export {}
