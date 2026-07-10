import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import {
  runCaptureGeneration,
  validateAnalysis,
  validateValue,
  createCaptureSessionId,
} from './capturePipeline'
import { __resetAuthForTests } from '../auth/deviceAuth'
import { __resetPublicConfigForTests } from '../config/publicConfig'

const tinyPng =
  'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=='

function seedAuth() {
  localStorage.setItem('ap_device_id', 'dev-1')
  localStorage.setItem('ap_access_token', 'tok')
  localStorage.setItem('ap_token_expires_at', new Date(Date.now() + 3600_000).toISOString())
}

describe('capturePipeline', () => {
  beforeEach(() => {
    __resetAuthForTests()
    __resetPublicConfigForTests()
    localStorage.clear()
    seedAuth()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

  it('validates analysis and value ranges', () => {
    const a = validateAnalysis({ breed: 'Tabby', quality_score: 99, clarity: -1 })
    expect(a.quality_score).toBe(10)
    expect(a.clarity).toBe(0)
    const v = validateValue({ rarity: 9, hp: 1, class: 'Nope', element: 'Nope', narrative: 'x' })
    expect(v.rarity).toBe(5)
    expect(v.hp).toBe(10)
    expect(v.class).toBe('Ranger')
    expect(v.element).toBe('Wind')
  })

  it('runs analyze → value with stable idempotency keys', async () => {
    const stages: string[] = []
    const fetchMock = vi.fn().mockImplementation(async (url: string, init?: RequestInit) => {
      const u = String(url)
      if (u.includes('/vision/analyze')) {
        return {
          ok: true,
          status: 200,
          headers: new Headers({ 'Content-Type': 'application/json' }),
          json: async () => ({
            breed: 'British Shorthair',
            color: 'blue',
            body_type: 'stocky',
            quality_score: 8,
            inference_id: 'inf-analyze-1',
            subject_completeness: 9,
            clarity: 8,
            lighting: 7,
            composition: 8,
            pose: 6,
            angle: 7,
          }),
        }
      }
      if (u.includes('/value/generate')) {
        return {
          ok: true,
          status: 200,
          headers: new Headers({ 'Content-Type': 'application/json' }),
          json: async () => ({
            rarity: 3,
            hp: 60,
            atk: 20,
            def: 18,
            spd: 22,
            class: 'Ranger',
            element: 'Wind',
            narrative: 'swift and alert',
            inference_id: 'inf-value-1',
          }),
        }
      }
      return {
        ok: false,
        status: 404,
        headers: new Headers(),
        json: async () => ({ error: 'not found' }),
      }
    })
    vi.stubGlobal('fetch', fetchMock)

    const sessionId = createCaptureSessionId()
    const animal = await runCaptureGeneration({
      sessionId,
      species: 'cat',
      photoDataUrl: tinyPng,
      onProgress: (p) => stages.push(p.stage),
    })

    expect(animal.species).toBe('cat')
    expect(animal.analysis.breed).toBe('British Shorthair')
    expect(animal.value.rarity).toBe(3)
    expect(stages).toContain('analyze')
    expect(stages).toContain('value')
    expect(stages).toContain('done')

    const analyzeCall = fetchMock.mock.calls.find((c) => String(c[0]).includes('/vision/analyze'))
    const valueCall = fetchMock.mock.calls.find((c) => String(c[0]).includes('/value/generate'))
    expect(analyzeCall?.[1]?.headers?.['Idempotency-Key']).toBe(`analyze:${sessionId}`)
    expect(valueCall?.[1]?.headers?.['Idempotency-Key']).toBe(`value:${sessionId}`)
    expect(analyzeCall?.[1]?.headers?.Authorization).toBe('Bearer tok')
  })

  it('resumes from analysis and only calls value', async () => {
    const fetchMock = vi.fn().mockImplementation(async (url: string) => {
      if (String(url).includes('/value/generate')) {
        return {
          ok: true,
          status: 200,
          headers: new Headers({ 'Content-Type': 'application/json' }),
          json: async () => ({
            rarity: 2,
            hp: 50,
            atk: 15,
            def: 15,
            spd: 15,
            class: 'Tank',
            element: 'Earth',
            narrative: 'steady',
            inference_id: 'inf-value-2',
          }),
        }
      }
      throw new Error(`unexpected call ${url}`)
    })
    vi.stubGlobal('fetch', fetchMock)

    const animal = await runCaptureGeneration({
      sessionId: 'sess-resume',
      species: 'dog',
      photoDataUrl: tinyPng,
      resumeFrom: {
        analysis: validateAnalysis({ breed: 'Shiba' }),
      },
    })
    expect(animal.analysis.breed).toBe('Shiba')
    expect(animal.value.class).toBe('Tank')
    expect(fetchMock.mock.calls.every((c) => String(c[0]).includes('/value/generate'))).toBe(true)
  })
})
