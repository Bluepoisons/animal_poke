import { openDB, type IDBPDatabase } from 'idb'

export const DB_NAME = 'animal-poke-db'
export const DB_VERSION = 1

let dbPromise: Promise<IDBPDatabase> | null = null

/** 获取 DB 实例（单例，惰性初始化） */
export function getDB(): Promise<IDBPDatabase> {
  if (!dbPromise) {
    dbPromise = openDB(DB_NAME, DB_VERSION, {
      upgrade(db) {
        // animals store：动物收藏数据
        if (!db.objectStoreNames.contains('animals')) {
          const store = db.createObjectStore('animals', { keyPath: 'id' })
          store.createIndex('by-date', 'captureDate')
          store.createIndex('by-rarity', 'rarity')
          // 注意：unlocked 是 boolean，不是合法的 IndexedDB key，不能建索引
        }
        // settings store：应用设置（单条记录）
        if (!db.objectStoreNames.contains('settings')) {
          db.createObjectStore('settings', { keyPath: 'key' })
        }
      },
    })
  }
  return dbPromise
}

/** 重置 DB 单例（仅供测试使用） */
export async function resetDB(): Promise<void> {
  if (dbPromise) {
    const db = await dbPromise
    db.close()
    dbPromise = null
  }
  indexedDB.deleteDatabase(DB_NAME)
}
