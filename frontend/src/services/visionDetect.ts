import type { SpeciesType } from '../types'

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
  detect: (photoData: string) => Promise<DetectionResult>
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

/** 模拟网络延迟范围 ms */
const MOCK_LATENCY: [number, number] = [300, 1200]

/** 模拟置信度范围 */
const MOCK_CONFIDENCE: [number, number] = [0.70, 0.98]

/** 三种物种均概率 */
const SPECIES_POOL: SpeciesType[] = ['cat', 'goose', 'dog']

/** 模拟检测框（随机位置） */
function randomBoundingBox(): [number, number, number, number] {
  const x = 0.15 + Math.random() * 0.2   // 0.15~0.35
  const y = 0.20 + Math.random() * 0.15  // 0.20~0.35
  const w = 0.3 + Math.random() * 0.2    // 0.30~0.50
  const h = 0.3 + Math.random() * 0.15   // 0.30~0.45
  return [x, y, w, h]
}

/** Mock 视觉检测器 —— 设计为接口形式，方便后续替换为真实 API */
export const mockVisionDetector: VisionDetector = {
  async detect(_photoData: string): Promise<DetectionResult> {
    // 模拟网络延迟
    const latency = MOCK_LATENCY[0] + Math.random() * (MOCK_LATENCY[1] - MOCK_LATENCY[0])
    await new Promise(resolve => setTimeout(resolve, latency))

    // 随机选择物种
    const species = SPECIES_POOL[Math.floor(Math.random() * SPECIES_POOL.length)]

    // 随机置信度
    const confidence = MOCK_CONFIDENCE[0] + Math.random() * (MOCK_CONFIDENCE[1] - MOCK_CONFIDENCE[0])

    // 随机检测框
    const boundingBox = randomBoundingBox()

    return { species, confidence: Math.round(confidence * 100) / 100, boundingBox }
  },
}

// ===== 未来替换为真实 API 的接口 =====
//
// export const apiVisionDetector: VisionDetector = {
//   async detect(photoData: string): Promise<DetectionResult> {
//     const formData = new FormData()
//     const blob = await (await fetch(photoData)).blob()
//     formData.append('frame', blob, 'frame.jpg')
//
//     const res = await fetch(`${import.meta.env.VITE_BACKEND_URL}/vision/detect`, {
//       method: 'POST',
//       headers: { Authorization: `Bearer ${getToken()}` },
//       body: formData,
//     })
//
//     if (!res.ok) throw new Error(`Vision detect failed: ${res.status}`)
//     return res.json()
//   },
// }
