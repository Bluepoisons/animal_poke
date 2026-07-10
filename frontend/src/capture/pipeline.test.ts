import { describe, it, expect, vi, beforeEach } from 'vitest'
import { runAnalyzeValuePipeline } from './pipeline'
import { createCaptureSession } from './session'
import * as auth from '../auth/deviceAuth'

describe('analyze-value pipeline', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    vi.spyOn(auth, 'getAccessToken').mockResolvedValue('tok')
  })

  it('runs analyze then value with distinct idempotency keys', async () => {
    const keys: string[] = []
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(async (_url: string, init: RequestInit) => {
        keys.push(String((init.headers as Record<string, string>)['Idempotency-Key']))
        return new Response(JSON.stringify({ ok: true, stage: keys.length }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }),
    )
    const session = createCaptureSession({ species: 'cat' })
    const stages: string[] = []
    const result = await runAnalyzeValuePipeline(session, (p) => stages.push(p.stage))
    expect(result).toBeTruthy()
    expect(keys[0]).toContain(':analyze')
    expect(keys[1]).toContain(':value')
    expect(stages).toContain('analyze')
    expect(stages).toContain('done')
  })
})
