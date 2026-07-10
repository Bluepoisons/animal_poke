import { getDB } from '../db'
import type { SyncQueueItem, SyncStatus } from '../types'

const STORE = 'sync_queue'

export const SyncQueueRepository = {
  async put(item: SyncQueueItem): Promise<void> {
    const db = await getDB()
    await db.put(STORE, item)
  },

  async getById(id: string): Promise<SyncQueueItem | undefined> {
    const db = await getDB()
    return db.get(STORE, id)
  },

  async getByIdempotencyKey(key: string): Promise<SyncQueueItem | undefined> {
    const db = await getDB()
    return db.getFromIndex(STORE, 'by-idempotency', key)
  },

  async getAll(): Promise<SyncQueueItem[]> {
    const db = await getDB()
    return db.getAll(STORE)
  },

  async getByStatus(status: SyncStatus): Promise<SyncQueueItem[]> {
    const db = await getDB()
    return db.getAllFromIndex(STORE, 'by-status', status)
  },

  /** 取出到期可发送的 pending/failed 项 */
  async listReady(now = Date.now()): Promise<SyncQueueItem[]> {
    const all = await this.getAll()
    return all
      .filter((i) => (i.status === 'pending' || i.status === 'failed') && i.nextAttemptAt <= now)
      .sort((a, b) => a.createdAt - b.createdAt)
  },

  async delete(id: string): Promise<void> {
    const db = await getDB()
    await db.delete(STORE, id)
  },

  async clearSynced(): Promise<number> {
    const synced = await this.getByStatus('synced')
    const db = await getDB()
    const tx = db.transaction(STORE, 'readwrite')
    for (const item of synced) {
      await tx.store.delete(item.id)
    }
    await tx.done
    return synced.length
  },
}
