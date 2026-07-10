import type { SpeciesType } from '../types'
import { authedRequest } from '../auth/deviceAuth'

// ===== 检测结果类型 =====

export interface DetectionResult {
  /** 检测到的物种 */
  species: SpeciesType
  /** 置信度 0~1 */
  confidence: number
  /** 归一化检测框 [x, y, width, height]，值范围 0~1 */
  boundingBox: [number, number, number, number]
  /** 服务端 inference id（若有） */
  inferenceId?: string
  /** 原始 label */
  label?: string
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

/** 物种差异化 VLM 置信度阈值：鹅容易与鸭/天鹅混淆，阈值降为 0.75 */
export const SPECIES_THRESHOLDS: Record<SpeciesType, number> = {
  cat: 0.85,
  goose: 0.75,
  dog: 0.85,
}

/** 获取指定物种的置信度阈值 */
export function getSpeciesThreshold(species: SpeciesType): number {
  return SPECIES_THRESHOLDS[species]
}

// ===== Mock 实现 =====

const MOCK_LATENCY: [number, number] = [300, 1200]
const MOCK_CONFIDENCE: [number, number] = [0.7, 0.98]
const SPECIES_POOL: SpeciesType[] = ['cat', 'goose', 'dog']

function randomBoundingBox(): [number, number, number, number] {
  const x = 0.15 + Math.random() * 0.2
  const y = 0.2 + Math.random() * 0.15
  const w = 0.3 + Math.random() * 0.2
  const h = 0.3 + Math.random() * 0.15
  return [x, y, w, h]
}

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
      boundingBox: randomBoundingBox(),
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
    confidence?: number
    bounding_box?: {
      x?: number
      y?: number
      w?: number
      h?: number
      width?: number
      height?: number
    }
  }>
  inference_id?: string
  inferenceId?: string
  request_id?: string
  degraded?: boolean
  source?: string
}

/** 权威可捕获物种；未知不默认为鹅 */
export type TaxonomySpecies = SpeciesType | 'unknown' | 'unsupported'

const CAPTURABLE: SpeciesType[] = ['cat', 'dog', 'goose']

/**
 * 将后端/模型原始标签映射为权威物种。
 * 有限别名表；鸭/天鹅/鸟/人/玩偶/空标签 → null（不进入捕获），禁止默认 goose。
 */
export function mapSpecies(raw?: string): SpeciesType | null {
  const original = (raw || '').trim()
  const s = original.toLowerCase().replace(/_/g, ' ')
  if (!s) return null

  const unsupportedExact = new Set([
    'bird', 'duck', 'swan', 'chicken', 'rooster', 'hen', 'pigeon', 'dove', 'parrot',
    'human', 'person', 'people', 'man', 'woman', 'child', 'baby',
    'toy', 'doll', 'plush', 'statue', 'screen', 'phone',
    '鸟', '鸭', '鸭子', '天鹅', '鸡', '人', '人类', '玩偶', '玩具', '屏幕',
  ])
  if (unsupportedExact.has(s)) return null
  if (['duck', 'swan', 'bird', 'chicken', 'human', 'person', 'toy', 'doll', 'screen', '鸭', '天鹅', '鸟', '人', '玩偶', '屏幕'].some((k) => s.includes(k))) {
    return null
  }

  if (s === 'cat' || s === 'kitten' || s === 'feline' || s.includes('猫')) return 'cat'
  if (s.includes('cat') && !s.includes('cattle') && !s.includes('caterpillar')) return 'cat'

  if (s === 'dog' || s === 'puppy' || s === 'canine' || s.includes('狗') || s.includes('犬')) return 'dog'
  if (s.includes('dog')) return 'dog'

  if (s === 'goose' || s === 'geese' || s === 'gander' || s === 'gosling') return 'goose'
  if ((s.includes('goose') || s.includes('geese') || s.includes('鹅')) && !s.includes('mongoose')) return 'goose'

  return null
}

export function isCapturableSpecies(s: string | null | undefined): s is SpeciesType {
  return !!s && (CAPTURABLE as string[]).includes(s)
}

function mapBackendAnimals(data: BackendDetectResponse): MultiDetectionResult {
  const inferenceId =
    data.inference_id ||
    data.inferenceId ||
    data.request_id ||
    (typeof crypto !== 'undefined' && 'randomUUID' in crypto
      ? crypto.randomUUID()
      : `detect-${Date.now()}`)
  const animals: DetectionResult[] = []
  for (const a of data.animals || []) {
    const mapped = mapSpecies(a.species || a.label)
    if (!mapped) {
      // 未知/不支持：保留 label 仅用于审计，不进入捕获列表
      continue
    }
    const bb = a.bounding_box || {}
    animals.push({
      species: mapped,
      confidence: Math.round((a.confidence || 0) * 1000) / 1000,
      boundingBox: [bb.x ?? 0, bb.y ?? 0, bb.w ?? bb.width ?? 0.3, bb.h ?? bb.height ?? 0.3],
      label: a.label || a.species,
      inferenceId,
    })
  }
  animals.sort((x, y) => y.confidence - x.confidence || x.species.localeCompare(y.species))
  return {
    animals,
    inferenceId,
    degraded: data.degraded || data.source === 'mock',
    source: data.source,
  }
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
