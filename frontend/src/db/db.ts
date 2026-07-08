import { openDB, type IDBPDatabase } from 'idb'

export const DB_NAME = 'animal-poke-db'
export const DB_VERSION = 2

let dbPromise: Promise<IDBPDatabase> | null = null

/** 获取 DB 实例（单例，惰性初始化） */
export function getDB(): Promise<IDBPDatabase> {
  if (!dbPromise) {
    dbPromise = openDB(DB_NAME, DB_VERSION, {
      async upgrade(db, oldVersion, _newVersion, transaction) {
        // v1: create stores + initial indexes
        if (!db.objectStoreNames.contains('animals')) {
          const store = db.createObjectStore('animals', { keyPath: 'id' })
          store.createIndex('by-date', 'captureDate')
          store.createIndex('by-rarity', 'rarity')
        }
        if (!db.objectStoreNames.contains('settings')) {
          db.createObjectStore('settings', { keyPath: 'key' })
        }
        // v2: add by-unlocked index + migrate existing records
        if (oldVersion < 2) {
          const store = transaction.objectStore('animals')
          if (!store.indexNames.contains('by-unlocked')) {
            store.createIndex('by-unlocked', 'isUnlocked')
          }
          // Migrate: add isUnlocked numeric field (0/1) to existing records
          let cursor = await store.openCursor()
          while (cursor) {
            const record = cursor.value
            record.isUnlocked = record.unlocked ? 1 : 0
            await cursor.update(record)
            cursor = await cursor.continue()
          }
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
