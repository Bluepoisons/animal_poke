/**
 * 可靠同步队列：IndexedDB pending → POST /sync/animal
 * - 稳定 Idempotency-Key
 * - 指数退避
 * - 409 按 reason_code 分类：duplicate/idempotency→synced；inference_*→永久失败
 * - 刷新不丢队列
 */

import { SyncQueueRepository } from '../db/repositories/sync-queue-repository'
import type { AnimalSyncPayload, SyncQueueItem } from '../db/types'
import { ApiError } from '../api/client'
import { authedRequest } from '../auth/deviceAuth'
import type { GeneratedAnimal } from './capturePipeline'
import { classifySync409, extractReasonCode } from './syncConflict'
import { pullAnimalsFromServer } from './syncPull'

const MAX_ATTEMPTS = 8
const BASE_DELAY_MS = 1000

function backoffMs(attempts: number): number {
  const exp = Math.min(attempts, 6)
  const jitter = Math.floor(Math.random() * 200)
  return BASE_DELAY_MS * 2 ** exp + jitter
}

function makeId(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return `sq-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`
}

export function buildIdempotencyKey(uuid: string): string {
  return `sync:animal:${uuid}`
}

export function generatedAnimalToPayload(
  animal: GeneratedAnimal,
  coords?: { lat?: number; lng?: number },
): AnimalSyncPayload {
  return {
    uuid: animal.sessionId,
    species: animal.species,
    breed: animal.analysis.breed,
    rarity: animal.value.rarity,
    hp: animal.value.hp,
    atk: animal.value.atk,
    def: animal.value.def,
    spd: animal.value.spd,
    class: animal.value.class,
    element: animal.value.element,
    latitude: coords?.lat,
    longitude: coords?.lng,
    generated_at: new Date().toISOString(),
    // 只传 value 阶段服务端 inference_id
    inference_request_id: animal.valueInferenceId || animal.inferenceRequestId,
    narrative: animal.value.narrative,
  }
}

/** 入队；同一 uuid/idempotencyKey 不重复创建 */
export async function enqueueAnimalSync(
  payload: AnimalSyncPayload,
  opts?: { animalId?: string },
): Promise<SyncQueueItem> {
  const idempotencyKey = buildIdempotencyKey(payload.uuid)
  const existing = await SyncQueueRepository.getByIdempotencyKey(idempotencyKey)
  if (existing) {
    if (existing.status === 'synced') return existing
    // 重置失败项以便立即重试
    const updated: SyncQueueItem = {
      ...existing,
      payload,
      animalId: opts?.animalId ?? existing.animalId,
      status: existing.status === 'syncing' ? 'syncing' : 'pending',
      nextAttemptAt: Date.now(),
      updatedAt: Date.now(),
      lastError: existing.status === 'failed' ? undefined : existing.lastError,
    }
    await SyncQueueRepository.put(updated)
    return updated
  }

  const now = Date.now()
  const item: SyncQueueItem = {
    id: makeId(),
    idempotencyKey,
    route: '/sync/animal',
    status: 'pending',
    attempts: 0,
    createdAt: now,
    updatedAt: now,
    nextAttemptAt: now,
    payload,
    animalId: opts?.animalId,
  }
  await SyncQueueRepository.put(item)
  return item
}

export async function enqueueGeneratedAnimal(
  animal: GeneratedAnimal,
  coords?: { lat?: number; lng?: number },
): Promise<SyncQueueItem> {
  return enqueueAnimalSync(generatedAnimalToPayload(animal, coords))
}

let flushing = false

/**
 * 冲洗队列。网络恢复或启动时可调用。
 * 返回本次成功同步数量。
 */
export async function flushSyncQueue(now = Date.now()): Promise<{ synced: number; failed: number }> {
  if (flushing) return { synced: 0, failed: 0 }
  flushing = true
  let synced = 0
  let failed = 0
  try {
    const ready = await SyncQueueRepository.listReady(now)
    for (const item of ready) {
      const ok = await processQueueItem(item)
      if (ok) synced += 1
      else failed += 1
    }
  } finally {
    flushing = false
  }
  return { synced, failed }
}

async function processQueueItem(item: SyncQueueItem): Promise<boolean> {
  const working: SyncQueueItem = {
    ...item,
    status: 'syncing',
    updatedAt: Date.now(),
  }
  await SyncQueueRepository.put(working)

  try {
    await authedRequest({
      method: 'POST',
      path: '/api/v1/sync/animal',
      body: JSON.stringify(working.payload),
      idempotencyKey: working.idempotencyKey,
      timeoutMs: 20_000,
      allowRetry: false,
    })
    const done: SyncQueueItem = {
      ...working,
      status: 'synced',
      updatedAt: Date.now(),
      lastError: undefined,
    }
    await SyncQueueRepository.put(done)
    return true
  } catch (err) {
    if (err instanceof ApiError && err.status === 409) {
      const reason = extractReasonCode(
        { reason_code: err.reasonCode, error: err.message },
        err.message,
      )
      const disposition = classifySync409(reason)
      if (disposition === 'treat_synced') {
        const done: SyncQueueItem = {
          ...working,
          status: 'synced',
          updatedAt: Date.now(),
          lastError: undefined,
        }
        await SyncQueueRepository.put(done)
        return true
      }
      if (disposition === 'permanent_fail') {
        const permanent: SyncQueueItem = {
          ...working,
          attempts: working.attempts + 1,
          status: 'failed',
          lastError: `${reason}: ${err.message}`,
          nextAttemptAt: Number.MAX_SAFE_INTEGER,
          updatedAt: Date.now(),
        }
        await SyncQueueRepository.put(permanent)
        return false
      }
      // unknown_conflict → 可重试
      const attempts = working.attempts + 1
      const next: SyncQueueItem = {
        ...working,
        attempts,
        status: attempts >= MAX_ATTEMPTS ? 'failed' : 'pending',
        lastError: `${reason}: ${err.message}`,
        nextAttemptAt: Date.now() + backoffMs(attempts),
        updatedAt: Date.now(),
      }
      await SyncQueueRepository.put(next)
      return false
    }

    const attempts = working.attempts + 1
    const message = err instanceof Error ? err.message : 'sync failed'
    const next: SyncQueueItem = {
      ...working,
      attempts,
      status: attempts >= MAX_ATTEMPTS ? 'failed' : 'pending',
      lastError: message,
      nextAttemptAt: Date.now() + backoffMs(attempts),
      updatedAt: Date.now(),
    }
    await SyncQueueRepository.put(next)
    return false
  }
}

/** 安装 online 监听：恢复网络时自动 flush */
export function installSyncOnlineFlush(): () => void {
  if (typeof window === 'undefined') return () => {}
  const onOnline = () => {
    void (async () => {
      try {
        await pullAnimalsFromServer()
      } catch {
        /* ignore */
      }
      await flushSyncQueue()
    })()
  }
  window.addEventListener('online', onOnline)
  return () => window.removeEventListener('online', onOnline)
}

export async function getSyncQueueSnapshot(): Promise<SyncQueueItem[]> {
  return SyncQueueRepository.getAll()
}
