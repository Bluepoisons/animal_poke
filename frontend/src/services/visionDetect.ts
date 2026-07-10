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
}

export interface VisionDetector {
  /** 对单帧照片进行动物检测 */
  detect: (photoData: string | Blob) => Promise<DetectionResult>
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
  async detect(_photoData: string | Blob): Promise<DetectionResult> {
    const latency = MOCK_LATENCY[0] + Math.random() * (MOCK_LATENCY[1] - MOCK_LATENCY[0])
    await new Promise((resolve) => setTimeout(resolve, latency))
    const species = SPECIES_POOL[Math.floor(Math.random() * SPECIES_POOL.length)]
    const confidence = MOCK_CONFIDENCE[0] + Math.random() * (MOCK_CONFIDENCE[1] - MOCK_CONFIDENCE[0])
    return {
      species,
      confidence: Math.round(confidence * 100) / 100,
      boundingBox: randomBoundingBox(),
    }
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
}

function mapSpecies(raw?: string): SpeciesType {
  const s = (raw || '').toLowerCase()
  if (s.includes('cat') || s.includes('猫')) return 'cat'
  if (s.includes('dog') || s.includes('狗')) return 'dog'
  return 'goose'
}

/** 真实 /api/v1/vision/detect（multipart field: image） */
export const apiVisionDetector: VisionDetector = {
  async detect(photoData: string | Blob): Promise<DetectionResult> {
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
    const animals = data.animals || []
    if (animals.length === 0) {
      throw new Error('no_animals_detected')
    }
    const best = [...animals].sort((a, b) => (b.confidence || 0) - (a.confidence || 0))[0]
    const bb = best.bounding_box || {}
    return {
      species: mapSpecies(best.species || best.label),
      confidence: Math.round((best.confidence || 0) * 1000) / 1000,
      boundingBox: [bb.x ?? 0, bb.y ?? 0, bb.w ?? bb.width ?? 0.3, bb.h ?? bb.height ?? 0.3],
    }
  },
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
