/**
 * 生产功能开关（AP-042 / #203）
 *
 * 默认关闭未完成能力，避免 UI 展示假成功入口。
 * 可通过运行时 window.__AP_CONFIG__.features 或 Vite env 覆盖（仅非敏感布尔）。
 *
 * 当 API 返回 reason_code=feature_unavailable 时，调用方应 markFeatureUnavailable。
 */

export type FeatureKey = 'achievements' | 'dispatch' | 'ranking' | 'pvp' | 'social' | 'ops'

export type FeatureFlagState = Record<FeatureKey, boolean>

const DEFAULT_FLAGS: FeatureFlagState = {
  achievements: true,
  dispatch: false, // 未完成：隐藏入口
  ranking: false,
  pvp: false,
  social: false,
  ops: false,
}


function envBool(key: string): boolean | undefined {
  const v = (import.meta.env as Record<string, string | undefined>)[key]
  if (v === undefined || v === '') return undefined
  const s = String(v).toLowerCase()
  if (['1', 'true', 'yes', 'on'].includes(s)) return true
  if (['0', 'false', 'no', 'off'].includes(s)) return false
  return undefined
}

function resolveFlags(): FeatureFlagState {
  const runtime = typeof window !== 'undefined' ? window.__AP_CONFIG__?.features : undefined
  return {
    achievements: runtime?.achievements ?? envBool('VITE_FEATURE_ACHIEVEMENTS') ?? DEFAULT_FLAGS.achievements,
    dispatch: runtime?.dispatch ?? envBool('VITE_FEATURE_DISPATCH') ?? DEFAULT_FLAGS.dispatch,
    ranking: runtime?.ranking ?? envBool('VITE_FEATURE_RANKING') ?? DEFAULT_FLAGS.ranking,
    pvp: runtime?.pvp ?? envBool('VITE_FEATURE_PVP') ?? DEFAULT_FLAGS.pvp,
    social: runtime?.social ?? envBool('VITE_FEATURE_SOCIAL') ?? DEFAULT_FLAGS.social,
    ops: runtime?.ops ?? envBool('VITE_FEATURE_OPS') ?? DEFAULT_FLAGS.ops,
  }
}

/** 静态初始快照（构建/运行时配置） */
export const FEATURE_FLAGS: FeatureFlagState = resolveFlags()

/** 运行时动态关闭：API 返回 feature_unavailable 后标记，隐藏入口 */
const runtimeDisabled = new Set<FeatureKey>()

export function isFeatureEnabled(key: FeatureKey): boolean {
  if (runtimeDisabled.has(key)) return false
  return FEATURE_FLAGS[key] === true
}

/** 后端返回 feature_unavailable 时调用，隐藏对应入口 */
export function markFeatureUnavailable(key: FeatureKey | string): void {
  const k = key as FeatureKey
  if (k in DEFAULT_FLAGS) {
    runtimeDisabled.add(k)
  }
}

/** 识别 API 错误是否为 feature_unavailable */
export function isFeatureUnavailableError(err: unknown): boolean {
  if (!err || typeof err !== 'object') return false
  const e = err as { reasonCode?: string; reason_code?: string; status?: number }
  const code = e.reasonCode || e.reason_code
  if (code === 'feature_unavailable') return true
  // 501/503 且 message 含 feature unavailable
  if ((e.status === 501 || e.status === 503) && 'message' in e) {
    const msg = String((e as { message?: string }).message || '').toLowerCase()
    if (msg.includes('feature unavailable') || msg.includes('not ready')) return true
  }
  return false
}

/** 从 API 路径推断 feature key */
export function featureKeyFromPath(path: string): FeatureKey | null {
  if (path.includes('/ranking/')) return 'ranking'
  if (path.includes('/pvp/')) return 'pvp'
  if (path.includes('/social/')) return 'social'
  if (path.includes('/ops/')) return 'ops'
  return null
}

/** 测试辅助 */
export function __resetFeatureFlagsForTests(): void {
  runtimeDisabled.clear()
}

export function __setRuntimeDisabledForTests(key: FeatureKey, disabled: boolean): void {
  if (disabled) runtimeDisabled.add(key)
  else runtimeDisabled.delete(key)
}
