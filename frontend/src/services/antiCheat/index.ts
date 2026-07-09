export type {
  EmulatorSignal,
  EmulatorCheckResult,
  RootCheckResult,
  LocationProof,
  TimeSyncResult,
  DeviceFingerprint,
  CameraProof,
  MotionSample,
  DeviceSecurityReport,
  DeviceRiskLevel,
  RiskAssessment,
} from './types'

export { detectEmulator } from './emulatorDetect'
export { checkRootStatus, checkRootStatusAsync } from './rootDetect'
export { analyzeLocation } from './locationVerify'
export { checkTimeSync } from './timeVerify'
export { collectFingerprint } from './fingerprint'
export {
  collectCameraProof,
  isVirtualCamera,
  startMotionSampling,
  analyzeMotionVariance,
} from './cameraVerify'
export { collectSecurityReport, evaluateRisk } from './report'
