/**
 * 捕获后 Analyze → Value 管线
 * - 阶段：upload → analyze → value → save
 * - 同 Session 稳定 Idempotency-Key + inferenceRequestId
 * - 失败只重试失败阶段；可取消（AbortSignal）
 */

import { ApiError } from '../api/client'
import { authedRequest } from '../auth/deviceAuth'

export type GenerationStage = 'idle' | 'upload' | 'analyze' | 'value' | 'save' | 'done' | 'error' | 'cancelled'

export const VALID_CLASSES = [
  'Warrior',
  'Mage',
  'Ranger',
  'Tank',
  'Support',
  'Assassin',
] as const

export const VALID_ELEMENTS = [
  'Fire',
  'Water',
  'Grass',
  'Electric',
  'Ice',
  'Dark',
  'Light',
  'Earth',
  'Wind',
] as const

export type PetClass = (typeof VALID_CLASSES)[number]
export type PetElement = (typeof VALID_ELEMENTS)[number]

export interface AnalysisResult {
  breed: string
  color: string
  body_type: string
  quality_score: number
  subject_completeness: number
  clarity: number
  lighting: number
  composition: number
  pose: number
  angle: number
  /** 服务端 analyze 阶段 inference_id */
  inference_id?: string
  model?: string
  prompt_version?: string
  source?: string
  degraded?: boolean
}

export interface ValueFactors {
  photo_quality: number
  completeness: number
  species_weight: number
  breed_weight: number
  color_weight: number
  base_score: number
  random_jitter: number
  final_score: number
  roll?: number
  quality_band: string
  config_version?: string
  seed_id?: string
}

export interface ValueResult {
  rarity: number // 1-5
  hp: number
  atk: number
  def: number
  spd: number
  class: string
  element: string
  narrative: string
  fiction?: boolean
  disclaimer?: string
  layer?: string
  /** 生成依据（拍摄质量/完整度/物种规则/有限随机） */
  factors?: ValueFactors
  config_version?: string
  seed_id?: string
  /** 服务端 value 阶段 inference_id（同步必须用此 ID） */
  inference_id?: string
  model?: string
  prompt_version?: string
  source?: string
  degraded?: boolean
}

export interface GeneratedAnimal {
  sessionId: string
  /** @deprecated 兼容字段；同步请用 valueInferenceId */
  inferenceRequestId: string
  /** detect 阶段 id（若有） */
  detectInferenceId?: string
  analyzeInferenceId?: string
  /** 权威：value 阶段服务端返回的 inference_id */
  valueInferenceId: string
  species: string
  analysis: AnalysisResult
  value: ValueResult
  photoDataUrl?: string
}

export interface PipelineProgress {
  stage: GenerationStage
  /** 0-100 */
  percent: number
  message: string
  error?: string
  result?: GeneratedAnimal
  /** 已完成的阶段缓存，便于只重试失败阶段 */
  partial?: {
    analysis?: AnalysisResult
    value?: ValueResult
  }
}

export interface RunPipelineOptions {
  sessionId: string
  species: string
  photoDataUrl: string
  /** detect 推理 id，用于服务端校验照片识别链路。 */
  detectInferenceId?: string
  signal?: AbortSignal
  onProgress?: (p: PipelineProgress) => void
  /** 从 partial 恢复（只跑未完成阶段） */
  resumeFrom?: {
    analysis?: AnalysisResult
    value?: ValueResult
  }
}

function clampInt(n: unknown, min: number, max: number, fallback: number): number {
  const v = typeof n === 'number' ? n : Number(n)
  if (!Number.isFinite(v)) return fallback
  return Math.min(max, Math.max(min, Math.round(v)))
}

function asString(v: unknown, fallback = ''): string {
  return typeof v === 'string' && v.trim() ? v.trim() : fallback
}

function pickInferenceId(o: Record<string, unknown>): string | undefined {
  const v =
    o.inference_id ?? o.inferenceId ?? o.inference_request_id ?? o.request_id
  return typeof v === 'string' && v.trim() ? v.trim() : undefined
}

export function validateAnalysis(raw: unknown): AnalysisResult {
  const o = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>
  return {
    breed: asString(o.breed, 'Unknown'),
    color: asString(o.color, 'unknown'),
    body_type: asString(o.body_type, 'normal'),
    quality_score: clampInt(o.quality_score, 1, 10, 5),
    subject_completeness: clampInt(o.subject_completeness, 0, 10, 5),
    clarity: clampInt(o.clarity, 0, 10, 5),
    lighting: clampInt(o.lighting, 0, 10, 5),
    composition: clampInt(o.composition, 0, 10, 5),
    pose: clampInt(o.pose, 0, 10, 5),
    angle: clampInt(o.angle, 0, 10, 5),
    inference_id: pickInferenceId(o),
    model: asString(o.model, '') || undefined,
    prompt_version: asString(o.prompt_version ?? o.promptVersion, '') || undefined,
    source: asString(o.source, '') || undefined,
    degraded: typeof o.degraded === 'boolean' ? o.degraded : undefined,
  }
}

function parseFactors(raw: unknown): ValueFactors | undefined {
  if (!raw || typeof raw !== 'object') return undefined
  const f = raw as Record<string, unknown>
  const num = (v: unknown, fb = 0) => {
    const n = typeof v === 'number' ? v : Number(v)
    return Number.isFinite(n) ? n : fb
  }
  return {
    photo_quality: num(f.photo_quality),
    completeness: num(f.completeness),
    species_weight: num(f.species_weight),
    breed_weight: num(f.breed_weight),
    color_weight: num(f.color_weight),
    base_score: num(f.base_score),
    random_jitter: num(f.random_jitter),
    final_score: num(f.final_score),
    roll: f.roll === undefined ? undefined : num(f.roll),
    quality_band: asString(f.quality_band, 'good'),
    config_version: asString(f.config_version, '') || undefined,
    seed_id: asString(f.seed_id, '') || undefined,
  }
}

export function validateValue(raw: unknown): ValueResult {
  const o = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>
  let petClass = asString(o.class, 'Ranger')
  if (!(VALID_CLASSES as readonly string[]).includes(petClass)) petClass = 'Ranger'
  let element = asString(o.element, 'Wind')
  if (!(VALID_ELEMENTS as readonly string[]).includes(element)) element = 'Wind'
  return {
    rarity: clampInt(o.rarity, 1, 5, 1),
    hp: clampInt(o.hp, 10, 100, 50),
    atk: clampInt(o.atk, 5, 50, 15),
    def: clampInt(o.def, 5, 50, 15),
    spd: clampInt(o.spd, 5, 50, 15),
    class: petClass,
    element,
    narrative: asString(o.narrative, 'A mysterious companion found in the wild.'),
    fiction: typeof o.fiction === 'boolean' ? o.fiction : undefined,
    disclaimer: asString(o.disclaimer, '') || undefined,
    layer: asString(o.layer, '') || undefined,
    factors: parseFactors(o.factors),
    config_version: asString(o.config_version ?? o.configVersion, '') || undefined,
    seed_id: asString(o.seed_id ?? o.seedId, '') || undefined,
    inference_id: pickInferenceId(o),
    model: asString(o.model, '') || undefined,
    prompt_version: asString(o.prompt_version ?? o.promptVersion, '') || undefined,
    source: asString(o.source, '') || undefined,
    degraded: typeof o.degraded === 'boolean' ? o.degraded : undefined,
  }
}

/** dataURL → Blob（analyze 上传用；不依赖 Response.blob，兼容 jsdom） */
export async function dataUrlToBlob(dataUrl: string): Promise<Blob> {
  if (dataUrl.startsWith('data:')) {
    const comma = dataUrl.indexOf(',')
    if (comma < 0) throw new Error('invalid data URL')
    const meta = dataUrl.slice(5, comma) // e.g. image/png;base64
    const payload = dataUrl.slice(comma + 1)
    const isBase64 = meta.includes(';base64')
    const mime = meta.split(';')[0] || 'application/octet-stream'
    if (isBase64) {
      const binary = atob(payload)
      const bytes = new Uint8Array(binary.length)
      for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i)
      return new Blob([bytes], { type: mime })
    }
    return new Blob([decodeURIComponent(payload)], { type: mime })
  }
  const res = await fetch(dataUrl)
  const buf = await res.arrayBuffer()
  const type = res.headers.get('content-type') || 'application/octet-stream'
  return new Blob([buf], { type })
}

function stagePercent(stage: GenerationStage): number {
  switch (stage) {
    case 'idle':
      return 0
    case 'upload':
      return 10
    case 'analyze':
      return 35
    case 'value':
      return 70
    case 'save':
      return 90
    case 'done':
      return 100
    case 'error':
    case 'cancelled':
      return 0
    default:
      return 0
  }
}

function stageMessage(stage: GenerationStage): string {
  switch (stage) {
    case 'upload':
      return '准备影像…'
    case 'analyze':
      return '深度分析中…'
    case 'value':
      return '生成属性与叙事…'
    case 'save':
      return '整理结果…'
    case 'done':
      return '生成完成'
    case 'cancelled':
      return '已取消'
    case 'error':
      return '生成失败'
    default:
      return ''
  }
}

function throwIfAborted(signal?: AbortSignal): void {
  if (signal?.aborted) {
    const err = new Error('pipeline cancelled')
    err.name = 'AbortError'
    throw err
  }
}

/**
 * 运行 Analyze → Value 管线。
 * 同一 sessionId 下 Idempotency-Key 稳定，重放不会重复计费语义（后端需配合）。
 */
export async function runCaptureGeneration(
  options: RunPipelineOptions,
): Promise<GeneratedAnimal> {
  const { sessionId, species, photoDataUrl, detectInferenceId, signal, onProgress, resumeFrom } = options
  const analyzeKey = `analyze:${sessionId}`
  const valueKey = `value:${sessionId}`

  let analysis = resumeFrom?.analysis
  let value = resumeFrom?.value
  let analyzeInferenceId = analysis?.inference_id
  let valueInferenceId = value?.inference_id

  const emit = (stage: GenerationStage, extra?: Partial<PipelineProgress>) => {
    onProgress?.({
      stage,
      percent: stagePercent(stage),
      message: stageMessage(stage),
      partial: { analysis, value },
      ...extra,
    })
  }

  try {
    // ---- upload / prepare ----
    if (!analysis) {
      emit('upload')
      throwIfAborted(signal)
      const blob = await dataUrlToBlob(photoDataUrl)
      throwIfAborted(signal)

      emit('analyze')
      const form = new FormData()
      form.append('image', blob, 'capture.jpg')
      form.append('species', species)
      if (detectInferenceId) form.append('detect_inference_id', detectInferenceId)

      const raw = await authedRequest<unknown>({
        method: 'POST',
        path: '/api/v1/vision/analyze',
        body: form,
        idempotencyKey: analyzeKey,
        signal,
        timeoutMs: 60_000,
        allowRetry: true,
      })
      analysis = validateAnalysis(raw)
      analyzeInferenceId = analysis.inference_id
    }

    // ---- value ----
    if (!value) {
      emit('value', { partial: { analysis, value } })
      throwIfAborted(signal)
      // 优先传递 analyze 阶段真实 inference_id；禁止用 sessionId 冒充
      const parentInference = analyzeInferenceId || analysis?.inference_id
      const payload = {
        species,
        breed: analysis!.breed,
        color: analysis!.color,
        body_type: analysis!.body_type,
        subject_completeness: analysis!.subject_completeness,
        clarity: analysis!.clarity,
        lighting: analysis!.lighting,
        composition: analysis!.composition,
        pose: analysis!.pose,
        angle: analysis!.angle,
        ...(parentInference
          ? { parent_inference_id: parentInference, analyze_inference_id: parentInference }
          : {}),
      }
      const raw = await authedRequest<unknown>({
        method: 'POST',
        path: '/api/v1/value/generate',
        body: JSON.stringify(payload),
        idempotencyKey: valueKey,
        signal,
        timeoutMs: 60_000,
        allowRetry: true,
      })
      value = validateValue(raw)
      valueInferenceId = value.inference_id
    }

    // ---- save (local assemble; sync 由 #103) ----
    emit('save', { partial: { analysis, value } })
    throwIfAborted(signal)

    // 同步权威 ID = value 阶段服务端返回；缺失则不可伪装为 sessionId
    const authoritativeId = valueInferenceId || value?.inference_id || ''
    if (!authoritativeId) {
      const err = new Error('value response missing inference_id')
      emit('error', { error: err.message, partial: { analysis, value } })
      throw err
    }

    const result: GeneratedAnimal = {
      sessionId,
      inferenceRequestId: authoritativeId,
      analyzeInferenceId: analyzeInferenceId || analysis?.inference_id,
      valueInferenceId: authoritativeId,
      species,
      analysis: analysis!,
      value: value!,
      photoDataUrl,
    }
    emit('done', { result, partial: { analysis, value } })
    return result
  } catch (err) {
    if (err instanceof Error && (err.name === 'AbortError' || signal?.aborted)) {
      emit('cancelled', { error: 'cancelled', partial: { analysis, value } })
      throw err
    }
    const message =
      err instanceof ApiError
        ? err.message
        : err instanceof Error
          ? err.message
          : 'generation failed'
    emit('error', { error: message, partial: { analysis, value } })
    throw err
  }
}

export function createCaptureSessionId(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return `cap-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`
}
