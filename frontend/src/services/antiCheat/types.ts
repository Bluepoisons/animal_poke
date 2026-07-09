/** 模拟器检测信号 */
export interface EmulatorSignal {
  name: string
  value: string
  suspicious: boolean
  weight: number
}

/** 模拟器检测结果 */
export interface EmulatorCheckResult {
  isEmulator: boolean
  signals: EmulatorSignal[]
  riskScore: number
}

/** Root/越狱检测结果 */
export interface RootCheckResult {
  isRooted: boolean
  signals: string[]
  riskScore: number
}

/** 位置欺骗检测结果 */
export interface LocationProof {
  lat: number
  lng: number
  accuracy: number
  timestamp: number
  clientTimestamp: number
  mockDetected: boolean
  mockSignals: string[]
}

/** 时间同步检测结果 */
export interface TimeSyncResult {
  clientTime: number
  serverTime: number
  offset: number
  isManipulated: boolean
}

/** 设备指纹 */
export interface DeviceFingerprint {
  uaHash: string
  screenHash: string
  gpuHash: string
  localeHash: string
  hardwareHash: string
  fingerprint: string
}

/** 相机证明数据 */
export interface CameraProof {
  trackLabel: string
  trackSettings: MediaTrackSettings | Record<string, unknown>
  trackState: string
  frameTimestamps: number[]
  sessionNonce: string
}

/** 运动传感器样本 */
export interface MotionSample {
  timestamp: number
  accelX: number
  accelY: number
  accelZ: number
  rotationAlpha: number
  rotationBeta: number
  rotationGamma: number
}

/** 完整设备安全报告 */
export interface DeviceSecurityReport {
  emulatorCheck: EmulatorCheckResult
  rootCheck: RootCheckResult
  locationProof: Partial<LocationProof>
  timeSync: Partial<TimeSyncResult>
  fingerprint: DeviceFingerprint
  collectedAt: number
}

/** 设备风险等级 */
export type DeviceRiskLevel = 'low' | 'medium' | 'high' | 'critical'

/** 风险评估结果 */
export interface RiskAssessment {
  level: DeviceRiskLevel
  score: number
  report: DeviceSecurityReport
}
