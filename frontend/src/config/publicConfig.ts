/**
 * 前端公开配置（仅非敏感字段）。
 * 第三方 Key / JWT Secret / DB 凭据严禁进入 Vite 环境变量。
 */

export type PublicConfig = {
  /** API 根地址；空字符串表示同域 + Vite 代理（开发） */
  apiBaseUrl: string
  logLevel: 'DEBUG' | 'INFO' | 'WARN' | 'ERROR'
  appEnv: 'development' | 'staging' | 'production' | 'test'
}

const FORBIDDEN_ENV_KEYS = [
  'VITE_JWT_SECRET',
  'VITE_TENCENT_MAP_KEY',
  'VITE_CAIYUN',
  'VITE_VISION_KEY',
  'VITE_LLM_KEY',
  'VITE_VLM_KEY',
  'VITE_DB_',
  'VITE_ADMIN_API_KEY',
]

function detectForbiddenEnv(): string[] {
  const hits: string[] = []
  // import.meta.env 在 Vite 中是静态可枚举的
  for (const key of Object.keys(import.meta.env)) {
    const upper = key.toUpperCase()
    if (FORBIDDEN_ENV_KEYS.some((f) => upper.includes(f.replace('VITE_', '')) && upper.startsWith('VITE_'))) {
      // 更严格：直接匹配禁止前缀
    }
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
      // validate
      // eslint-disable-next-line no-new
      new URL(v)
      return v
    } catch {
      throw new Error(`Invalid VITE_API_BASE_URL: ${raw}`)
    }
  }
  throw new Error(`VITE_API_BASE_URL must be absolute URL or path starting with /, got: ${raw}`)
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
    mode === 'production' || mode === 'staging' || mode === 'test' ? (mode as PublicConfig['appEnv']) : 'development'

  const logRaw = (import.meta.env.VITE_LOG_LEVEL || 'INFO').toUpperCase()
  const logLevel = (['DEBUG', 'INFO', 'WARN', 'ERROR'].includes(logRaw)
    ? logRaw
    : 'INFO') as PublicConfig['logLevel']

  cached = {
    apiBaseUrl: normalizeBaseUrl(import.meta.env.VITE_API_BASE_URL as string | undefined),
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
