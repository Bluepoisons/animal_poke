import type { CardEntry } from '../types'

/** 动物收藏记录（继承 CardEntry，后续可扩展 DB 专属字段） */
export interface AnimalRecord extends CardEntry {
  /** Numeric flag for IndexedDB indexing (0 = locked, 1 = unlocked).
   *  Auto-synced with `unlocked` boolean on add/update. */
  isUnlocked?: number
}

/** 应用设置（单条记录，key 固定为 'prefs'） */
export interface AppSettings {
  key: 'prefs'
  soundEnabled: boolean
  musicEnabled: boolean
  privacyConsented: boolean
  /** AP-053 extended prefs (optional for back-compat) */
  hapticsEnabled?: boolean
  reduceMotion?: boolean
  dataSaver?: boolean
  locale?: string
}

/** 同步队列状态 */
export type SyncStatus = 'pending' | 'syncing' | 'synced' | 'failed'

/** 待同步的动物载荷（对齐后端 POST /sync/animal） */
export interface AnimalSyncPayload {
  uuid: string
  species: string
  breed?: string
  rarity: number
  hp?: number
  atk?: number
  def?: number
  spd?: number
  class?: string
  element?: string
  latitude?: number
  longitude?: number
  generated_at: string
  inference_request_id?: string
  narrative?: string
}

/** IndexedDB 同步队列项 */
export interface SyncQueueItem {
  id: string
  /** 业务幂等键：device+route+uuid 维度，客户端稳定生成 */
  idempotencyKey: string
  route: '/sync/animal'
  status: SyncStatus
  attempts: number
  lastError?: string
  createdAt: number
  updatedAt: number
  nextAttemptAt: number
  payload: AnimalSyncPayload
  /** 可选本地关联的图鉴 id */
  animalId?: string
}
