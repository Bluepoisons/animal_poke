interface FetchRetryOptions {
  retries?: number
  retryDelay?: number
  timeout?: number
  signal?: AbortSignal
}

/**
 * 带重试 + 超时的 fetch 封装。
 * 仅对网络错误和 5xx 进行重试，4xx 不重试。
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
  } = retryOptions

  let lastError: Error | null = null

  for (let attempt = 0; attempt <= retries; attempt++) {
    try {
      const controller = new AbortController()
      const timeoutId = setTimeout(() => controller.abort(), timeout)

      if (retryOptions.signal) {
        retryOptions.signal.addEventListener('abort', () => controller.abort())
      }

      const response = await fetch(url, {
        ...options,
        signal: controller.signal,
      })

      clearTimeout(timeoutId)

      if (response.status >= 500 && attempt < retries) {
        await sleep(retryDelay * (attempt + 1))
        continue
      }

      return response
    } catch (err: unknown) {
      const e = err as Error
      lastError = e
      if (e.name === 'AbortError' && !e.message.includes('timeout')) {
        throw e
      }
      if (attempt < retries) {
        await sleep(retryDelay * (attempt + 1))
      }
    }
  }

  throw lastError ?? new Error('fetchWithRetry failed')
}

function sleep(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms))
}
