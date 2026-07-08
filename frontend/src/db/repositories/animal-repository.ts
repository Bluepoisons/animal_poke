import { getDB } from '../db'
import type { AnimalRecord } from '../types'

/** Helper: sync isUnlocked numeric field with unlocked boolean */
function withIndex(record: AnimalRecord): AnimalRecord {
  return { ...record, isUnlocked: record.unlocked ? 1 : 0 }
}

/** 动物收藏数据访问层 */
export const AnimalRepository = {
  /** 获取所有动物 */
  async getAll(): Promise<AnimalRecord[]> {
    const db = await getDB()
    return db.getAll('animals')
  },

  /** 按 ID 获取单个动物 */
  async getById(id: string): Promise<AnimalRecord | undefined> {
    const db = await getDB()
    return db.get('animals', id)
  },

  /** 仅获取已解锁的动物（使用 by-unlocked 索引，避免全表扫描） */
  async getUnlocked(): Promise<AnimalRecord[]> {
    const db = await getDB()
    return db.getAllFromIndex('animals', 'by-unlocked', 1)
  },

  /** 按日期范围筛选（闭区间，ISO 日期字符串比较） */
  async getByDateRange(start: string, end: string): Promise<AnimalRecord[]> {
    const db = await getDB()
    const range = IDBKeyRange.bound(start, end)
    return db.getAllFromIndex('animals', 'by-date', range)
  },

  /** 新增一只动物（自动同步 isUnlocked 索引字段） */
  async add(entry: AnimalRecord): Promise<void> {
    const db = await getDB()
    await db.add('animals', withIndex(entry))
  },

  /** 更新动物信息（自动同步 isUnlocked 索引字段） */
  async update(entry: AnimalRecord): Promise<void> {
    const db = await getDB()
    await db.put('animals', withIndex(entry))
  },

  /** 标记为已查看（isNew = false），使用 cursor update 减少一次 IO */
  async markViewed(id: string): Promise<void> {
    const db = await getDB()
    const tx = db.transaction('animals', 'readwrite')
    const cursor = await tx.store.openCursor(id)
    if (cursor) {
      const record = cursor.value
      record.isNew = false
      await cursor.update(record)
    }
    await tx.done
  },

  /** 删除动物 */
  async delete(id: string): Promise<void> {
    const db = await getDB()
    await db.delete('animals', id)
  },

  /** 获取已解锁总数（使用 by-unlocked 索引 count，避免全表加载） */
  async countUnlocked(): Promise<number> {
    const db = await getDB()
    return db.countFromIndex('animals', 'by-unlocked', 1)
  },

  /** 批量写入（用于初始化 mock 数据，自动同步 isUnlocked） */
  async bulkAdd(entries: AnimalRecord[]): Promise<void> {
    const db = await getDB()
    const tx = db.transaction('animals', 'readwrite')
    await Promise.all(entries.map(entry => tx.store.add(withIndex(entry))))
    await tx.done
  },

  /** 分页查询（游标跳过 + 读取） */
  async getPage(offset: number, limit: number): Promise<AnimalRecord[]> {
    const db = await getDB()
    const tx = db.transaction('animals', 'readonly')
    let cursor = await tx.store.openCursor()
    // Skip offset records
    for (let i = 0; i < offset && cursor; i++) {
      cursor = await cursor.continue()
    }
    // Read limit records
    const results: AnimalRecord[] = []
    for (let i = 0; i < limit && cursor; i++) {
      results.push(cursor.value)
      cursor = await cursor.continue()
    }
    await tx.done
    return results
  },
}
