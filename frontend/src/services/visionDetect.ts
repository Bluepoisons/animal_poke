import type { SpeciesType } from '../types'
import {
  capturableSpeciesIds,
  findSpeciesIdByLabel,
  getDetectThreshold,
  isCapturableSpecies as isRegisteredCapturableSpecies,
} from '../species'
import { authedRequest } from '../auth/deviceAuth'

// ===== 检测结果类型 =====

export interface DetectionResult {
  /** 检测到的物种 */
  species: SpeciesType
  /** 置信度 0~1 */
  confidence: number
  /** 服务端 inference id（若有） */
  inferenceId?: string
  /** 原始 label */
  label?: string
  /** 服务端检测凭证中的稳定目标 ID。 */
  targetId?: string
}

/** 多目标检测响应 */
export interface MultiDetectionResult {
  animals: DetectionResult[]
  inferenceId: string
  degraded?: boolean
  source?: string
}

export interface VisionDetector {
  /** 对单帧照片进行动物检测（取置信度最高的一只，兼容旧调用） */
  detect: (photoData: string | Blob) => Promise<DetectionResult>
  /** 返回全部候选 + inferenceId */
  detectAll?: (photoData: string | Blob) => Promise<MultiDetectionResult>
}

// ===== 物种差异化置信度阈值 =====

/** 物种差异化 VLM 置信度阈值（内容包 detect_threshold） */
export const SPECIES_THRESHOLDS: Record<string, number> = Object.fromEntries(
  capturableSpeciesIds().map((id) => [id, getDetectThreshold(id)]),
)

/** 获取指定物种的置信度阈值 */
export function getSpeciesThreshold(species: SpeciesType): number {
  return SPECIES_THRESHOLDS[species] ?? getDetectThreshold(species)
}

// ===== Mock 实现 =====

const MOCK_LATENCY: [number, number] = [300, 1200]
const MOCK_CONFIDENCE: [number, number] = [0.7, 0.98]
// other_animal 必须携带一个经服务端确认的具体中文动物名，不能随机裸生成。
const SPECIES_POOL: SpeciesType[] = capturableSpeciesIds().filter((id) => id !== 'other_animal')

/** Mock 视觉检测器 —— 仅测试/显式开发开关 */
export const mockVisionDetector: VisionDetector = {
  async detectAll(_photoData: string | Blob): Promise<MultiDetectionResult> {
    const latency = MOCK_LATENCY[0] + Math.random() * (MOCK_LATENCY[1] - MOCK_LATENCY[0])
    await new Promise((resolve) => setTimeout(resolve, latency))
    const species = SPECIES_POOL[Math.floor(Math.random() * SPECIES_POOL.length)]
    const confidence = MOCK_CONFIDENCE[0] + Math.random() * (MOCK_CONFIDENCE[1] - MOCK_CONFIDENCE[0])
    const animal: DetectionResult = {
      species,
      confidence: Math.round(confidence * 100) / 100,
      inferenceId: `mock-inf-${Date.now()}`,
    }
    return {
      animals: [animal],
      inferenceId: animal.inferenceId!,
      source: 'mock',
    }
  },
  async detect(photoData: string | Blob): Promise<DetectionResult> {
    const all = await this.detectAll!(photoData)
    if (!all.animals.length) throw new Error('no_animals_detected')
    return { ...all.animals[0], inferenceId: all.inferenceId }
  },
}

function dataUrlToBlob(dataUrl: string): Blob {
  const parts = dataUrl.split(',')
  if (parts.length < 2) return new Blob([], { type: 'image/jpeg' })
  const mime = parts[0].match(/:(.*?);/)?.[1] || 'image/jpeg'
  const bin = atob(parts[1])
  const arr = new Uint8Array(bin.length)
  for (let i = 0; i < bin.length; i++) arr[i] = bin.charCodeAt(i)
  return new Blob([arr], { type: mime })
}

export async function blobOrDataToImageFile(photoData: string | Blob): Promise<Blob> {
  if (typeof photoData !== 'string') return photoData
  if (photoData.startsWith('data:')) return dataUrlToBlob(photoData)
  const res = await fetch(photoData)
  return res.blob()
}

type BackendDetectResponse = {
  animals?: Array<{
    species?: string
    label?: string
    target_id?: string
    confidence?: number
  }>
  targets?: Array<{
    species?: string
    label?: string
    target_id?: string
    confidence?: number
  }>
  inference_id?: string
  inferenceId?: string
  request_id?: string
  degraded?: boolean
  source?: string
}

/** 权威可捕获物种；未知不默认为任意动物 */
export type TaxonomySpecies = SpeciesType | 'unknown' | 'unsupported'

const REJECTED_ENGLISH_TOKENS = new Set([
  'human', 'person', 'people', 'man', 'woman', 'child', 'baby',
  'toy', 'doll', 'plush', 'statue', 'screen', 'phone', 'smartphone',
])
const REJECTED_CHINESE_LABELS = ['人类', '人物', '行人', '男人', '女人', '儿童', '婴儿', '玩偶', '玩具', '毛绒', '雕像', '屏幕', '手机']

function isRejectedNonAnimalLabel(raw: string): boolean {
  const lower = raw.toLowerCase().replace(/[_-]+/g, ' ')
  const tokens = lower.match(/[a-z]+/g) ?? []
  if (tokens.some((token) => REJECTED_ENGLISH_TOKENS.has(token))) return true
  return REJECTED_CHINESE_LABELS.some((label) => raw.includes(label)) || raw.trim() === '人'
}

/**
 * 将后端/模型原始标签映射为权威物种。
 * 物种别名与 contains 规则来自 species registry；只显式拒绝人、玩具、屏幕和空标签。
 */
export function mapSpecies(raw?: string): SpeciesType | null {
  const original = (raw || '').trim()
  if (!original || isRejectedNonAnimalLabel(original)) return null
  if (original.toLowerCase() === 'other_animal') return 'other_animal'
  const mapped = findSpeciesIdByLabel(original)
  return mapped === 'other_animal' ? null : mapped
}

export function isCapturableSpecies(s: string | null | undefined): s is SpeciesType {
  return typeof s === 'string' && isRegisteredCapturableSpecies(s)
}

export function mapBackendAnimals(data: BackendDetectResponse): MultiDetectionResult {
  const inferenceId =
    data.inference_id ||
    data.inferenceId ||
    data.request_id ||
    (typeof crypto !== 'undefined' && 'randomUUID' in crypto
      ? crypto.randomUUID()
      : `detect-${Date.now()}`)
  const animals: DetectionResult[] = []
  const rawList = (data.targets && data.targets.length > 0 ? data.targets : data.animals) || []
  for (const a of rawList) {
    const mapped = mapSpecies(a.species || a.label)
    if (!mapped) {
      // 未知/不支持：保留 label 仅用于审计，不进入捕获列表
      continue
    }
    animals.push({
      species: mapped,
      confidence: Math.round((a.confidence || 0) * 1000) / 1000,
      label: a.label || a.species,
      targetId: a.target_id,
      inferenceId,
    })
  }
  animals.sort((x, y) => y.confidence - x.confidence || x.species.localeCompare(y.species))
  return {
    // The capture flow only needs one confirmed species.  Choosing the most
    // confident candidate keeps the VLM interaction photo-based and avoids a
    // second target-selection/box-drawing step.
    animals: animals.slice(0, 1),
    inferenceId,
    degraded: data.degraded || data.source === 'mock',
    source: data.source,
  }
}

type SpeciesCorrectionResponse = {
  inference_id: string
  parent_inference_id: string
  target_id: string
  original_species: string
  species: string
  label: string
  confidence?: number
  source: 'user_confirmation'
  expires_at?: string
}

/**
 * 将用户纠错写成一个可审计的派生 detect 凭证。
 * 成功前不得替换本地 detectInferenceId，否则 Analyze 会拒绝物种不一致。
 */
export async function confirmSpeciesCorrection(input: {
  detectInferenceId: string
  targetId?: string
  species: string
  speciesLabelZh?: string
}): Promise<SpeciesCorrectionResponse> {
  const label = input.speciesLabelZh?.trim()
  return authedRequest<SpeciesCorrectionResponse>({
    method: 'POST',
    path: '/api/v1/vision/detect/corrections',
    body: JSON.stringify({
      detect_inference_id: input.detectInferenceId,
      target_id: input.targetId,
      species: input.species,
      species_label_zh: label || undefined,
    }),
    idempotencyKey: `vision-correction:${input.detectInferenceId}:${input.targetId || 'primary'}:${input.species}:${label || ''}`,
    allowRetry: true,
    timeoutMs: 20_000,
  })
}

/** 真实 /api/v1/vision/detect（multipart field: image） */
export const apiVisionDetector: VisionDetector = {
  async detectAll(photoData: string | Blob): Promise<MultiDetectionResult> {
    const blob = await blobOrDataToImageFile(photoData)
    const form = new FormData()
    form.append('image', blob, 'frame.jpg')
    const data = await authedRequest<BackendDetectResponse>({
      method: 'POST',
      path: '/api/v1/vision/detect',
      body: form,
      timeoutMs: 30_000,
      allowRetry: true,
      idempotencyKey:
        typeof crypto !== 'undefined' && 'randomUUID' in crypto
          ? `vision-detect-${crypto.randomUUID()}`
          : `vision-detect-${Date.now()}`,
    })
    const mapped = mapBackendAnimals(data)
    if (mapped.animals.length === 0) {
      throw new Error('no_animals_detected')
    }
    return mapped
  },
  async detect(photoData: string | Blob): Promise<DetectionResult> {
    const all = await this.detectAll!(photoData)
    const best = [...all.animals].sort((a, b) => b.confidence - a.confidence)[0]
    return { ...best, inferenceId: all.inferenceId }
  },
}

/** 统一多目标检测入口 */
export async function detectAnimals(
  photoData: string | Blob,
  preferMock = import.meta.env.MODE === 'test' || import.meta.env.VITE_VISION_MOCK === '1',
): Promise<MultiDetectionResult> {
  const detector = getVisionDetector(preferMock)
  if (detector.detectAll) return detector.detectAll(photoData)
  const one = await detector.detect(photoData)
  return {
    animals: [one],
    inferenceId: one.inferenceId || `legacy-${Date.now()}`,
  }
}

/**
 * 生产默认真实检测。
 * - vitest: mock
 * - VITE_VISION_MOCK=1: mock
 */
export function getVisionDetector(
  preferMock = import.meta.env.MODE === 'test' || import.meta.env.VITE_VISION_MOCK === '1',
): VisionDetector {
  return preferMock ? mockVisionDetector : apiVisionDetector
}
