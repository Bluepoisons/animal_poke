import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mockVisionDetector, mapSpecies, isCapturableSpecies } from './visionDetect'
import type { SpeciesType } from '../types'

const VALID_SPECIES: SpeciesType[] = ['cat', 'goose', 'dog']

describe('visionDetect', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('detect 返回有效 species（cat/goose/dog 之一）', async () => {
    const promise = mockVisionDetector.detect('data:image/jpeg;base64,xxx')
    vi.advanceTimersByTime(2000)
    const result = await promise
    expect(VALID_SPECIES).toContain(result.species)
  })

  it('detect 返回置信度在 0~1 范围', async () => {
    const promise = mockVisionDetector.detect('data:image/jpeg;base64,xxx')
    vi.advanceTimersByTime(2000)
    const result = await promise
    expect(result.confidence).toBeGreaterThanOrEqual(0)
    expect(result.confidence).toBeLessThanOrEqual(1)
  })

  it('detect 只返回物种和置信度，不返回框坐标', async () => {
    const promise = mockVisionDetector.detect('data:image/jpeg;base64,xxx')
    vi.advanceTimersByTime(2000)
    const result = await promise
    expect(result).not.toHaveProperty('boundingBox')
  })

  it('detect 模拟网络延迟 ≥ 300ms', async () => {
    const start = Date.now()
    const promise = mockVisionDetector.detect('data:image/jpeg;base64,xxx')
    vi.advanceTimersByTime(300)
    vi.advanceTimersByTime(1000)
    await promise
    const elapsed = Date.now() - start
    expect(elapsed).toBeGreaterThanOrEqual(300)
  })

  it('多次调用结果有随机性', async () => {
    const results: string[] = []
    for (let i = 0; i < 5; i++) {
      const promise = mockVisionDetector.detect('data:image/jpeg;base64,xxx')
      vi.advanceTimersByTime(2000)
      const result = await promise
      results.push(`${result.species}_${result.confidence}`)
    }
    const unique = new Set(results)
    expect(unique.size).toBeGreaterThan(1)
  })
})

describe('mapSpecies taxonomy (AP-007)', () => {
  it('maps cat/dog/goose aliases including Chinese', () => {
    expect(mapSpecies('cat')).toBe('cat')
    expect(mapSpecies('小猫')).toBe('cat')
    expect(mapSpecies('dog')).toBe('dog')
    expect(mapSpecies('狗')).toBe('dog')
    expect(mapSpecies('goose')).toBe('goose')
    expect(mapSpecies('大鹅')).toBe('goose')
  })

  it('never maps duck/swan/bird/human/toy/empty to goose', () => {
    for (const raw of ['duck', 'swan', 'bird', '鸟', '鸭子', '天鹅', 'human', 'person', '人', 'toy', '玩偶', 'screen', '', '  ', 'horse']) {
      expect(mapSpecies(raw)).toBeNull()
    }
  })

  it('isCapturableSpecies only allows cat/dog/goose', () => {
    expect(isCapturableSpecies('cat')).toBe(true)
    expect(isCapturableSpecies('goose')).toBe(true)
    expect(isCapturableSpecies(null)).toBe(false)
    expect(isCapturableSpecies('bird')).toBe(false)
  })
})
