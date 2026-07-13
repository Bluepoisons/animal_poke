import { describe, it, expect, beforeEach } from 'vitest'
import { registerCapture, loadCollectionMeta } from './collectionValue'

describe('collectionValue', () => {
  beforeEach(() => localStorage.clear())
  it('first capture unlocks', () => {
    const r = registerCapture('cat')
    expect(r.isFirst).toBe(true)
    expect(loadCollectionMeta().cat.captureCount).toBe(1)
  })
  it('duplicates grant research not new species unlock message', () => {
    registerCapture('dog')
    const r = registerCapture('dog')
    expect(r.isFirst).toBe(false)
    expect(r.researchGained).toBeGreaterThan(0)
    expect(loadCollectionMeta().dog.captureCount).toBe(2)
  })

  it('keeps different concrete other animals as separate discoveries', () => {
    expect(registerCapture('other_animal', '赤狐').isFirst).toBe(true)
    expect(registerCapture('other_animal', '仓鼠').isFirst).toBe(true)
    expect(registerCapture('other_animal', '赤狐').isFirst).toBe(false)
    expect(loadCollectionMeta()['other_animal:赤狐'].captureCount).toBe(2)
    expect(loadCollectionMeta()['other_animal:仓鼠'].captureCount).toBe(1)
  })
})
