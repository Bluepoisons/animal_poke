import type { TimeSyncResult } from './types'

/**
 * 与服务端时间同步，检测设备时间篡改。
 * 使用 GET /api/v1/time（返回 unix_ms + X-Server-Time）。
 * 接口失败时标记 unknown，不默认为安全。
 */
export async function checkTimeSync(): Promise<TimeSyncResult> {
  const clientTime = Date.now()
  try {
    const resp = await fetch('/api/v1/time', { method: 'GET' })
    if (!resp.ok) {
      return {
        clientTime,
        serverTime: 0,
        offset: 0,
        isManipulated: false,
        // @ts-expect-error optional extension for callers
        unavailable: true,
      }
    }
    const serverTimeHeader = resp.headers.get('X-Server-Time')
    let serverTime = 0
    if (serverTimeHeader) {
      // 可能是 RFC3339 或毫秒时间戳
      const asInt = parseInt(serverTimeHeader, 10)
      if (!Number.isNaN(asInt) && asInt > 1e12) {
        serverTime = asInt
      } else {
        const parsed = Date.parse(serverTimeHeader)
        serverTime = Number.isNaN(parsed) ? 0 : parsed
      }
    }
    if (!serverTime) {
      const data = await resp.json().catch(() => null) as { unix_ms?: number; server_time?: string } | null
      if (data?.unix_ms) serverTime = data.unix_ms
      else if (data?.server_time) serverTime = Date.parse(data.server_time)
    }
    if (!serverTime) {
      return {
        clientTime,
        serverTime: 0,
        offset: 0,
        isManipulated: false,
        // @ts-expect-error optional
        unavailable: true,
      }
    }
    const offset = clientTime - serverTime
    return {
      clientTime,
      serverTime,
      offset,
      isManipulated: Math.abs(offset) > 5 * 60 * 1000,
    }
  } catch {
    // 网络不可用：不默认为绝对安全
    return {
      clientTime,
      serverTime: 0,
      offset: 0,
      isManipulated: false,
      // @ts-expect-error optional
      unavailable: true,
    }
  }
}
