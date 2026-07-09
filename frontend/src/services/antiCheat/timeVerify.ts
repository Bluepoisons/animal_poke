import type { TimeSyncResult } from './types'

/**
 * 与服务端时间同步，检测设备时间篡改。
 * 依赖后端在响应头注入 X-Server-Time。
 */
export async function checkTimeSync(): Promise<TimeSyncResult> {
  const clientTime = Date.now()
  try {
    const resp = await fetch('/api/v1/health', { method: 'GET' })
    const serverTimeHeader = resp.headers.get('X-Server-Time')
    const serverTime = serverTimeHeader
      ? parseInt(serverTimeHeader, 10)
      : Date.now()
    const offset = clientTime - serverTime
    // 偏差超过 5 分钟判定为可疑
    return {
      clientTime,
      serverTime,
      offset,
      isManipulated: Math.abs(offset) > 5 * 60 * 1000,
    }
  } catch {
    // 网络不可用时无法校验，返回不操纵
    return {
      clientTime,
      serverTime: clientTime,
      offset: 0,
      isManipulated: false,
    }
  }
}
