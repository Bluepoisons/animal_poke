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
})
