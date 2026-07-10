import 'fake-indexeddb/auto'
import { describe, it, expect, beforeEach } from 'vitest'
import { openDB } from 'idb'
import { DB_NAME, DB_VERSION, getDB, resetDB } from './db'

describe('IDB migration (AP-040)', () => {
  beforeEach(async () => {
    await resetDB()
  })

  it('opens at v3 with animals/settings/sync_queue', async () => {
    const db = await getDB()
    expect(db.version).toBe(DB_VERSION)
    expect([...db.objectStoreNames].sort()).toEqual(['animals', 'settings', 'sync_queue'].sort())
  })

  it('upgrades from v1 schema to v3 and preserves animal rows', async () => {
    // Seed a v1 database (no sync_queue, no by-unlocked)
    const v1 = await openDB(DB_NAME, 1, {
      upgrade(db) {
        const store = db.createObjectStore('animals', { keyPath: 'id' })
        store.createIndex('by-date', 'captureDate')
        store.createIndex('by-rarity', 'rarity')
        db.createObjectStore('settings', { keyPath: 'key' })
      },
    })
    await v1.put('animals', {
      id: 'legacy-1',
      captureDate: '2026-01-01T00:00:00.000Z',
      rarity: 'common',
      unlocked: true,
    })
    v1.close()

    // App opens at DB_VERSION=3 → upgrade path runs
    const db = await getDB()
    expect(db.version).toBe(3)
    expect(db.objectStoreNames.contains('sync_queue')).toBe(true)
    const row = await db.get('animals', 'legacy-1')
    expect(row).toBeTruthy()
    expect(row.id).toBe('legacy-1')
    // v2 migration adds numeric isUnlocked
    expect(row.isUnlocked).toBe(1)
  })
})
