import { getAccessToken } from '../auth/deviceAuth'
import { getApiBaseUrl } from '../api/client'

export type SyncStatus = 'pending' | 'syncing' | 'synced' | 'failed'

export type SyncItem = {
  id: string
  idempotencyKey: string
  payload: Record<string, unknown>
  status: SyncStatus
  attempts: number
  lastError?: string
  updatedAt: number
}

const STORAGE_KEY = 'ap_sync_queue_v1'

function load(): SyncItem[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return []
    const arr = JSON.parse(raw)
    return Array.isArray(arr) ? arr : []
  } catch {
    return []
  }
}

function save(items: SyncItem[]) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(items.slice(-200)))
}

export function enqueueSync(payload: Record<string, unknown>, idempotencyKey?: string): SyncItem {
  const items = load()
  const id =
    typeof crypto !== 'undefined' && 'randomUUID' in crypto
      ? crypto.randomUUID()
      : `sync-${Date.now()}`
  const item: SyncItem = {
    id,
    idempotencyKey: idempotencyKey || `sync-${id}`,
    payload,
    status: 'pending',
    attempts: 0,
    updatedAt: Date.now(),
  }
  items.push(item)
  save(items)
  return item
}

export function listSyncQueue(): SyncItem[] {
  return load()
}

export async function flushSyncQueue(signal?: AbortSignal): Promise<{ synced: number; failed: number }> {
  const items = load()
  let synced = 0
  let failed = 0
  const base = getApiBaseUrl()
  for (let i = 0; i < items.length; i++) {
    const item = items[i]
    if (item.status === 'synced') continue
    if (signal?.aborted) break
    item.status = 'syncing'
    item.attempts += 1
    item.updatedAt = Date.now()
    save(items)
    try {
      const token = await getAccessToken(signal)
      const res = await fetch(`${base}/api/v1/sync/animal`, {
        method: 'POST',
        headers: {
          Authorization: `Bearer ${token}`,
          'Content-Type': 'application/json',
          'Idempotency-Key': item.idempotencyKey,
          'X-Request-ID': crypto.randomUUID?.() || `s-${Date.now()}`,
        },
        body: JSON.stringify(item.payload),
        signal,
      })
      if (res.status === 409 || res.ok) {
        item.status = 'synced'
        item.lastError = undefined
        synced++
      } else if (res.status >= 500) {
        item.status = 'failed'
        item.lastError = `http_${res.status}`
        failed++
      } else {
        // 4xx 非 409：标记 failed 避免死循环
        item.status = 'failed'
        item.lastError = `http_${res.status}`
        failed++
      }
    } catch (e: unknown) {
      item.status = 'pending'
      item.lastError = (e as Error)?.message || 'network'
      failed++
    }
    item.updatedAt = Date.now()
    save(items)
  }
  return { synced, failed }
}

export function __resetSyncQueueForTests() {
  localStorage.removeItem(STORAGE_KEY)
}
