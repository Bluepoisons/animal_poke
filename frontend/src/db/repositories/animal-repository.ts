import { getDB } from '../db'
import type { AnimalRecord } from '../types'

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

  /** 仅获取已解锁的动物（unlocked 是 boolean，无法用索引，用 JS 过滤） */
  async getUnlocked(): Promise<AnimalRecord[]> {
    const db = await getDB()
    const all = await db.getAll('animals')
    return all.filter(a => a.unlocked)
  },

  /** 按日期范围筛选（闭区间，ISO 日期字符串比较） */
  async getByDateRange(start: string, end: string): Promise<AnimalRecord[]> {
    const db = await getDB()
    const range = IDBKeyRange.bound(start, end)
    return db.getAllFromIndex('animals', 'by-date', range)
  },

  /** 新增一只动物 */
  async add(entry: AnimalRecord): Promise<void> {
    const db = await getDB()
    await db.add('animals', entry)
  },

  /** 更新动物信息 */
  async update(entry: AnimalRecord): Promise<void> {
    const db = await getDB()
    await db.put('animals', entry)
  },

  /** 标记为已查看（isNew = false） */
  async markViewed(id: string): Promise<void> {
    const db = await getDB()
    const record = await db.get('animals', id)
    if (record) {
      record.isNew = false
      await db.put('animals', record)
    }
  },

  /** 删除动物 */
  async delete(id: string): Promise<void> {
    const db = await getDB()
    await db.delete('animals', id)
  },

  /** 获取已解锁总数 */
  async countUnlocked(): Promise<number> {
    const db = await getDB()
    const all = await db.getAll('animals')
    return all.filter(a => a.unlocked).length
  },

  /** 批量写入（用于初始化 mock 数据） */
  async bulkAdd(entries: AnimalRecord[]): Promise<void> {
    const db = await getDB()
    const tx = db.transaction('animals', 'readwrite')
    await Promise.all(entries.map(entry => tx.store.add(entry)))
    await tx.done
  },
}
