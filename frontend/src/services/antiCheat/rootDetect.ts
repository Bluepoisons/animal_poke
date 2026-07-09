import type { RootCheckResult } from './types'

/**
 * Root / 越狱检测 — Web 端间接检测。
 * 纯 PWA 模式下检测能力有限，Capacitor 原生插件可提供更强检测。
 */
export function checkRootStatus(): RootCheckResult {
  const signals: string[] = []

  // 1. webdriver 检测（自动化工具注入标志）
  if ((navigator as unknown as { webdriver?: boolean }).webdriver === true) {
    signals.push('navigator.webdriver=true')
  }

  // 2. 开发者工具检测（React DevTools 在生产环境出现）
  try {
    const devTools = (window as unknown as { __REACT_DEVTOOLS_GLOBAL_HOOK__?: unknown }).__REACT_DEVTOOLS_GLOBAL_HOOK__
    if (devTools && import.meta.env?.PROD) {
      signals.push('react_devtools_present_in_production')
    }
  } catch { /* noop */ }

  return {
    isRooted: signals.length > 0,
    signals,
    riskScore: signals.length > 0 ? 30 : 0,
  }
}

/**
 * 异步 Root 检测 — 尝试加载 Capacitor 原生插件。
 * 纯 PWA 模式下 fallback 为 Web 端检测结果。
 */
export async function checkRootStatusAsync(): Promise<RootCheckResult> {
  const webResult = checkRootStatus()

  try {
    // 动态导入 Capacitor 插件（如果安装了 @animalpoke/anticheat）
    // 使用变量构造 import 路径，防止 Vite 在构建时静态解析失败
    const moduleName = '@animalpoke/anticheat'
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const mod: any = await import(/* @vite-ignore */ moduleName)
    const nativeResult = await mod.RootCheck.check()
    return {
      isRooted: nativeResult.isRooted || webResult.isRooted,
      signals: [...webResult.signals, ...nativeResult.signals],
      riskScore: nativeResult.isRooted ? 100 : webResult.riskScore,
    }
  } catch {
    // Capacitor 插件不可用（纯 PWA 模式）
    return webResult
  }
}
