import { describe, it, expect } from 'vitest'
import {
  getPrologueSequence,
  prologueSlice,
  rhythmTable,
  validatePrologueSlice,
} from './blankPage'

describe('AP-124 prologue slice', () => {
  it('validates structure and budget', () => {
    expect(validatePrologueSlice()).toEqual([])
  })

  it('has four beats and three cast members', () => {
    expect(prologueSlice.beats).toHaveLength(4)
    expect(prologueSlice.castIds).toHaveLength(3)
    expect(prologueSlice.unlocksChapter).toBe('ch01.alley_echo')
  })

  it('choice sequence has multi-option value choice', () => {
    const seq = getPrologueSequence('prologue.choice')
    const choice = seq?.segments.find((s) => s.choice)?.choice
    expect(choice?.options.length).toBeGreaterThanOrEqual(2)
  })

  it('rhythm table sums near 15-20 minutes', () => {
    const sum = rhythmTable().reduce((a, b) => a + b.minutes, 0)
    expect(sum).toBeGreaterThanOrEqual(15)
    expect(sum).toBeLessThanOrEqual(22)
  })
})
