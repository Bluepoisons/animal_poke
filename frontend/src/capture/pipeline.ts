import { getAccessToken } from '../auth/deviceAuth'
import { getApiBaseUrl } from '../api/client'
import type { CaptureSession } from './session'

export type PipelineStage = 'upload' | 'analyze' | 'value' | 'save' | 'done' | 'error'

export type PipelineProgress = {
  stage: PipelineStage
  message: string
  analyze?: unknown
  value?: unknown
  error?: string
}

export type PipelineControllers = {
  signal?: AbortSignal
}

async function postJSON<T>(path: string, body: unknown, key: string, signal?: AbortSignal): Promise<T> {
  const token = await getAccessToken(signal)
  const base = getApiBaseUrl()
  const res = await fetch(`${base}${path}`, {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${token}`,
      'Content-Type': 'application/json',
      'Idempotency-Key': key,
      'X-Request-ID': crypto.randomUUID?.() || `p-${Date.now()}`,
    },
    body: JSON.stringify(body),
    signal,
  })
  if (!res.ok) {
    throw new Error(`${path} failed: ${res.status}`)
  }
  return res.json() as Promise<T>
}

/**
 * 捕获成功后 Analyze → Value 串联；按阶段幂等键，失败可从失败阶段重试。
 */
export async function runAnalyzeValuePipeline(
  session: CaptureSession,
  onProgress: (p: PipelineProgress) => void,
  ctrl: PipelineControllers = {},
): Promise<{ analyze: unknown; value: unknown }> {
  onProgress({ stage: 'analyze', message: '深度分析中…' })
  const analyze = await postJSON(
    '/api/v1/vision/analyze',
    {
      species: session.species,
      client_session_id: session.id,
      detection: session.detection,
    },
    `${session.idempotencyKey}:analyze`,
    ctrl.signal,
  )
  onProgress({ stage: 'value', message: '生成属性中…', analyze })
  const value = await postJSON(
    '/api/v1/value/generate',
    {
      species: session.species,
      client_session_id: session.id,
      analyze,
    },
    `${session.idempotencyKey}:value`,
    ctrl.signal,
  )
  onProgress({ stage: 'done', message: '生成完成', analyze, value })
  return { analyze, value }
}
