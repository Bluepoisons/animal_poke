import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mockVisionDetector, mapBackendAnimals, mapSpecies, isCapturableSpecies } from './visionDetect'
import type { SpeciesType } from '../types'
import { capturableSpeciesIds } from '../species'

const VALID_SPECIES: SpeciesType[] = capturableSpeciesIds()

describe('visionDetect', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('detect 返回注册表中的有效 species', async () => {
    const promise = mockVisionDetector.detect('data:image/jpeg;base64,xxx')
    vi.advanceTimersByTime(2000)
    const result = await promise
    expect(VALID_SPECIES).toContain(result.species)
    expect(result.species).not.toBe('other_animal')
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
  it('maps registry aliases and Chinese labels across animal groups', () => {
    expect(mapSpecies('cat')).toBe('cat')
    expect(mapSpecies('小猫')).toBe('cat')
    expect(mapSpecies('dog')).toBe('dog')
    expect(mapSpecies('狗')).toBe('dog')
    expect(mapSpecies('goose')).toBe('goose')
    expect(mapSpecies('大鹅')).toBe('goose')
    expect(mapSpecies('African elephant')).toBe('elephant')
    expect(mapSpecies('axolotl')).toBe('salamander')
    expect(mapSpecies('大型猫科动物')).toBe('big_cat')
  })

  it('keeps generic bird distinct while mapping specific birds', () => {
    expect(mapSpecies('bird')).toBe('bird')
    expect(mapSpecies('鸟类')).toBe('bird')
    expect(mapSpecies('蜂鸟')).toBe('bird')
    expect(mapSpecies('swan')).toBe('bird')
    expect(mapSpecies('duck')).toBe('duck')
    expect(mapSpecies('goose')).toBe('goose')
    expect(mapSpecies('parrot')).toBe('parrot')
  })

  it('avoids Chinese single-character substring collisions', () => {
    expect(mapSpecies('海马')).toBe('fish')
    expect(mapSpecies('牛蛙')).toBe('frog')
    expect(mapSpecies('食人鱼')).toBe('fish')
    expect(mapSpecies('河马')).toBeNull()
    expect(mapSpecies('蜗牛')).toBeNull()
    expect(mapSpecies('海牛')).toBeNull()
    expect(mapSpecies('木马')).toBeNull()
  })

  it('uses English word boundaries after exact aliases', () => {
    expect(mapSpecies('seahorse')).toBe('fish')
    expect(mapSpecies('a horse in grass')).toBe('horse')
    expect(mapSpecies('workhorse')).toBeNull()
    expect(mapSpecies('catfish')).toBeNull()
    expect(mapSpecies('caracal')).toBeNull()
  })

  it('rejects non-animal, unknown, unsupported and empty labels', () => {
    for (const raw of [
      'human', 'person', '人', 'toy dog', '玩偶', 'phone screen', '', '  ',
      'unknown', 'unsupported', 'plant', 'tree', 'car', 'fox', '狐狸', 'unknown creature',
      'other animal', 'unknown animal', 'unidentified animal',
    ]) {
      expect(mapSpecies(raw)).toBeNull()
    }
    expect(mapSpecies('other_animal')).toBe('other_animal')
  })

  it('preserves the backend specific Chinese label for other animals', () => {
    const mapped = mapBackendAnimals({
      inference_id: 'inf-other',
      animals: [{ species: 'other_animal', label: '狐狸', target_id: 'fox-7', confidence: 0.91 }],
    })
    expect(mapped.animals[0]).toMatchObject({
      species: 'other_animal',
      label: '狐狸',
      targetId: 'fox-7',
      confidence: 0.91,
    })
  })

  it('isCapturableSpecies follows the full registry', () => {
    expect(isCapturableSpecies('cat')).toBe(true)
    expect(isCapturableSpecies('goose')).toBe(true)
    expect(isCapturableSpecies('bird')).toBe(true)
    expect(isCapturableSpecies('whale')).toBe(true)
    expect(isCapturableSpecies('other_animal')).toBe(true)
    expect(isCapturableSpecies(null)).toBe(false)
    expect(isCapturableSpecies('dragon')).toBe(false)
  })
})
