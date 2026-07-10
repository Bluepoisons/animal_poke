export class FetchTimeoutError extends Error {
  constructor(message = 'request timeout') {
    super(message)
    this.name = 'FetchTimeoutError'
  }
}

export class FetchAbortedError extends Error {
  constructor(message = 'request aborted') {
    super(message)
    this.name = 'FetchAbortedError'
  }
}

export interface FetchRetryOptions {
  retries?: number
  retryDelay?: number
  timeout?: number
  signal?: AbortSignal
  /** HTTP method for idempotency decisions */
  method?: string
  /** Explicitly allow retrying non-idempotent methods (must pair with Idempotency-Key) */
  allowRetryOnWrite?: boolean
}

function isIdempotentMethod(method: string | undefined): boolean {
  const m = (method || 'GET').toUpperCase()
  return m === 'GET' || m === 'HEAD' || m === 'OPTIONS'
}

function sleep(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal?.aborted) {
      reject(new FetchAbortedError())
      return
    }
    const t = setTimeout(resolve, ms)
    const onAbort = () => {
      clearTimeout(t)
      reject(new FetchAbortedError())
    }
    signal?.addEventListener('abort', onAbort, { once: true })
  })
}

function backoffMs(base: number, attempt: number, retryAfterSec?: number): number {
  if (retryAfterSec && retryAfterSec > 0) return retryAfterSec * 1000
  const exp = base * Math.pow(2, attempt)
  const jitter = Math.floor(Math.random() * Math.min(250, exp * 0.2))
  return exp + jitter
}

/**
 * 带重试 + 超时的 fetch 封装。
 * - 区分 timeout 与 caller abort
 * - 默认只重试幂等方法；写请求需 allowRetryOnWrite
 * - finally 清理 timer / listener
 */
export async function fetchWithRetry(
  url: string,
  options: RequestInit = {},
  retryOptions: FetchRetryOptions = {},
): Promise<Response> {
  const {
    retries = 3,
    retryDelay = 1000,
    timeout = 10000,
    method = options.method || 'GET',
    allowRetryOnWrite = false,
  } = retryOptions

  const canRetry =
    isIdempotentMethod(method) || allowRetryOnWrite

  let lastError: Error | null = null
  const maxAttempts = canRetry ? retries : 0

  for (let attempt = 0; attempt <= maxAttempts; attempt++) {
    const controller = new AbortController()
    let timedOut = false
    let timeoutId: ReturnType<typeof setTimeout> | undefined
    let onParentAbort: (() => void) | undefined

    try {
      timeoutId = setTimeout(() => {
        timedOut = true
        controller.abort()
      }, timeout)

      if (retryOptions.signal) {
        if (retryOptions.signal.aborted) {
          throw new FetchAbortedError()
        }
        onParentAbort = () => controller.abort()
        retryOptions.signal.addEventListener('abort', onParentAbort)
      }

      const response = await fetch(url, {
        ...options,
        method,
        signal: controller.signal,
      })

      if (response.status === 429 || response.status >= 500) {
        if (attempt < maxAttempts && canRetry) {
          const ra = Number(response.headers.get('Retry-After') || '')
          await sleep(backoffMs(retryDelay, attempt, Number.isFinite(ra) ? ra : undefined), retryOptions.signal)
          continue
        }
      }

      return response
    } catch (err: unknown) {
      const e = err as Error
      if (retryOptions.signal?.aborted && !timedOut) {
        throw new FetchAbortedError(e.message)
      }
      if (timedOut) {
        lastError = new FetchTimeoutError()
        if (attempt < maxAttempts && canRetry) {
          await sleep(backoffMs(retryDelay, attempt), retryOptions.signal)
          continue
        }
        throw lastError
      }
      if (e.name === 'AbortError') {
        throw new FetchAbortedError(e.message)
      }
      lastError = e
      if (attempt < maxAttempts && canRetry) {
        await sleep(backoffMs(retryDelay, attempt), retryOptions.signal)
        continue
      }
    } finally {
      if (timeoutId) clearTimeout(timeoutId)
      if (onParentAbort && retryOptions.signal) {
        retryOptions.signal.removeEventListener('abort', onParentAbort)
      }
    }
  }

  throw lastError ?? new Error('fetchWithRetry failed')
}
