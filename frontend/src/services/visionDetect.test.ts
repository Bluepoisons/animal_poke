import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mockVisionDetector } from './visionDetect'
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

  it('detect 返回有效 boundingBox（4 个值都在 0~1 范围）', async () => {
    const promise = mockVisionDetector.detect('data:image/jpeg;base64,xxx')
    vi.advanceTimersByTime(2000)
    const result = await promise
    expect(result.boundingBox).toHaveLength(4)
    for (const v of result.boundingBox) {
      expect(v).toBeGreaterThanOrEqual(0)
      expect(v).toBeLessThanOrEqual(1)
    }
  })

  it('detect 模拟网络延迟 ≥ 300ms', async () => {
    const start = Date.now()
    const promise = mockVisionDetector.detect('data:image/jpeg;base64,xxx')
    // 推进 300ms
    vi.advanceTimersByTime(300)
    // 推进更多以确保完成
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
    // 5 次调用不应全相同（概率极低）
    const unique = new Set(results)
    expect(unique.size).toBeGreaterThan(1)
  })
})
