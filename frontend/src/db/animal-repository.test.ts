import { describe, it, expect, beforeEach } from 'vitest'
import 'fake-indexeddb/auto'
import { AnimalRepository } from './repositories/animal-repository'
import { resetDB } from './db'
import type { AnimalRecord } from './types'

// 每个测试前重置 DB 单例并清空数据库，确保隔离
beforeEach(async () => {
  await resetDB()
})

// 测试用 mock 数据
function makeEntry(overrides: Partial<AnimalRecord> = {}): AnimalRecord {
  return {
    id: 'test-1',
    no: '#000059',
    rarity: 'common',
    unlocked: true,
    captureDate: '2026-07-08',
    location: '海曙区·月湖',
    lat: 29.87,
    lng: 121.55,
    seed: 1,
    isNew: true,
    ...overrides,
  }
}

describe('AnimalRepository', () => {
  it('#1 add → getById 能读到', async () => {
    const entry = makeEntry()
    await AnimalRepository.add(entry)
    const result = await AnimalRepository.getById('test-1')
    expect(result).toBeDefined()
    expect(result?.id).toBe('test-1')
    expect(result?.no).toBe('#000059')
  })

  it('#2 getAll 返回全部', async () => {
    await AnimalRepository.add(makeEntry({ id: 'a1' }))
    await AnimalRepository.add(makeEntry({ id: 'a2' }))
    await AnimalRepository.add(makeEntry({ id: 'a3' }))
    const all = await AnimalRepository.getAll()
    expect(all).toHaveLength(3)
  })

  it('#3 getUnlocked 只返回 unlocked=true', async () => {
    await AnimalRepository.add(makeEntry({ id: 'a1', unlocked: true }))
    await AnimalRepository.add(makeEntry({ id: 'a2', unlocked: false }))
    await AnimalRepository.add(makeEntry({ id: 'a3', unlocked: true }))
    const unlocked = await AnimalRepository.getUnlocked()
    expect(unlocked).toHaveLength(2)
    expect(unlocked.every(a => a.unlocked)).toBe(true)
  })

  it('#4 getByDateRange 日期筛选', async () => {
    await AnimalRepository.add(makeEntry({ id: 'a1', captureDate: '2026-07-01' }))
    await AnimalRepository.add(makeEntry({ id: 'a2', captureDate: '2026-07-05' }))
    await AnimalRepository.add(makeEntry({ id: 'a3', captureDate: '2026-07-08' }))
    await AnimalRepository.add(makeEntry({ id: 'a4', captureDate: '2026-07-10' }))
    const range = await AnimalRepository.getByDateRange('2026-07-03', '2026-07-08')
    expect(range).toHaveLength(2)
    expect(range.map(a => a.id).sort()).toEqual(['a2', 'a3'])
  })

  it('#5 markViewed → isNew 变 false', async () => {
    await AnimalRepository.add(makeEntry({ id: 'a1', isNew: true }))
    await AnimalRepository.markViewed('a1')
    const result = await AnimalRepository.getById('a1')
    expect(result?.isNew).toBe(false)
  })

  it('#6 delete → getById 返回 undefined', async () => {
    await AnimalRepository.add(makeEntry({ id: 'a1' }))
    await AnimalRepository.delete('a1')
    const result = await AnimalRepository.getById('a1')
    expect(result).toBeUndefined()
  })

  it('#7 countUnlocked 计数', async () => {
    await AnimalRepository.add(makeEntry({ id: 'a1', unlocked: true }))
    await AnimalRepository.add(makeEntry({ id: 'a2', unlocked: false }))
    await AnimalRepository.add(makeEntry({ id: 'a3', unlocked: true }))
    const count = await AnimalRepository.countUnlocked()
    expect(count).toBe(2)
  })

  it('#8 bulkAdd 批量写入', async () => {
    const entries = [
      makeEntry({ id: 'b1' }),
      makeEntry({ id: 'b2' }),
      makeEntry({ id: 'b3' }),
      makeEntry({ id: 'b4' }),
      makeEntry({ id: 'b5' }),
    ]
    await AnimalRepository.bulkAdd(entries)
    const all = await AnimalRepository.getAll()
    expect(all).toHaveLength(5)
  })

  it('#9 空数据库 getAll 返回空数组', async () => {
    const all = await AnimalRepository.getAll()
    expect(all).toEqual([])
  })
})
