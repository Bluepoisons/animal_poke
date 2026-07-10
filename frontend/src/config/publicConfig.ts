/**
 * 前端公开配置（仅非敏感字段）。
 * 第三方 Key / JWT Secret / DB 凭据严禁进入 Vite 环境变量。
 *
 * 解析优先级：
 * 1. window.__AP_CONFIG__.apiBaseUrl（运行时 /config.js，生产 Nginx 注入）
 * 2. import.meta.env.VITE_API_BASE_URL（构建期）
 * 3. 空字符串 → 同域相对路径 /api/*（生产推荐）
 */

export type PublicConfig = {
  /** API 根地址；空字符串表示同域 + /api 反代（或 Vite dev proxy） */
  apiBaseUrl: string
  logLevel: 'DEBUG' | 'INFO' | 'WARN' | 'ERROR'
  appEnv: 'development' | 'staging' | 'production' | 'test'
}

declare global {
  interface Window {
    __AP_CONFIG__?: {
      apiBaseUrl?: string
      /** optional non-sensitive feature toggles injected at runtime */
      features?: Record<string, boolean>
    }
  }
}

function detectForbiddenEnv(): string[] {
  const hits: string[] = []
  for (const key of Object.keys(import.meta.env)) {
    if (
      /VITE_.*(JWT|SECRET|TENCENT|CAIYUN|VISION_KEY|LLM_KEY|VLM_KEY|ADMIN_API|DB_PASSWORD)/i.test(key)
    ) {
      hits.push(key)
    }
  }
  return hits
}

function normalizeBaseUrl(raw: string | undefined): string {
  if (!raw || !raw.trim()) return ''
  const v = raw.trim().replace(/\/$/, '')
  // 允许相对路径 /api 或绝对 http(s)
  if (v.startsWith('/')) return v
  if (v.startsWith('http://') || v.startsWith('https://')) {
    try {
      // eslint-disable-next-line no-new
      new URL(v)
      return v
    } catch {
      throw new Error(`Invalid API base URL: ${raw}`)
    }
  }
  throw new Error(`API base URL must be absolute URL or path starting with /, got: ${raw}`)
}

function resolveRawApiBaseUrl(): string | undefined {
  // Runtime config from /config.js (production nginx entrypoint)
  if (typeof window !== 'undefined' && window.__AP_CONFIG__) {
    const runtime = window.__AP_CONFIG__.apiBaseUrl
    if (runtime !== undefined && runtime !== null) {
      return String(runtime)
    }
  }
  return import.meta.env.VITE_API_BASE_URL as string | undefined
}

let cached: PublicConfig | null = null

/** 读取并校验公开配置；非法时抛错阻止启动 */
export function loadPublicConfig(): PublicConfig {
  if (cached) return cached
  const forbidden = detectForbiddenEnv()
  if (forbidden.length > 0) {
    throw new Error(
      `Forbidden sensitive Vite env keys detected: ${forbidden.join(', ')}. ` +
        'Third-party keys must stay on the Go backend only.',
    )
  }

  const mode = (import.meta.env.MODE || 'development').toLowerCase()
  const appEnv: PublicConfig['appEnv'] =
    mode === 'production' || mode === 'staging' || mode === 'test'
      ? (mode as PublicConfig['appEnv'])
      : 'development'

  const logRaw = (import.meta.env.VITE_LOG_LEVEL || 'INFO').toUpperCase()
  const logLevel = (['DEBUG', 'INFO', 'WARN', 'ERROR'].includes(logRaw)
    ? logRaw
    : 'INFO') as PublicConfig['logLevel']

  const apiBaseUrl = normalizeBaseUrl(resolveRawApiBaseUrl())

  // Production must not silently use an invalid absolute host; empty (same-origin) is OK.
  // Invalid values already throw in normalizeBaseUrl.
  if (
    appEnv === 'production' &&
    apiBaseUrl &&
    !apiBaseUrl.startsWith('/') &&
    !apiBaseUrl.startsWith('https://') &&
    !apiBaseUrl.startsWith('http://')
  ) {
    throw new Error(`Invalid production apiBaseUrl: ${apiBaseUrl}`)
  }

  cached = {
    apiBaseUrl,
    logLevel,
    appEnv,
  }
  return cached
}

export function getApiBaseUrl(): string {
  return loadPublicConfig().apiBaseUrl
}

/** 测试辅助 */
export function __resetPublicConfigForTests(): void {
  cached = null
}
