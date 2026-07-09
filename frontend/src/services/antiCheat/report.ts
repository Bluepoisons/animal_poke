import type {
  DeviceSecurityReport,
  RiskAssessment,
  DeviceRiskLevel,
} from './types'
import { detectEmulator } from './emulatorDetect'
import { checkRootStatusAsync } from './rootDetect'
import { checkTimeSync } from './timeVerify'
import { collectFingerprint } from './fingerprint'

/**
 * 采集完整设备安全报告。
 * 汇总所有检测模块的结果。
 */
export async function collectSecurityReport(): Promise<DeviceSecurityReport> {
  const [emulatorCheck, rootCheck, timeSync] = await Promise.all([
    Promise.resolve(detectEmulator()),
    checkRootStatusAsync(),
    checkTimeSync(),
  ])

  return {
    emulatorCheck,
    rootCheck,
    locationProof: {},
    timeSync,
    fingerprint: collectFingerprint(),
    collectedAt: Date.now(),
  }
}

/**
 * 评估设备风险等级。
 *
 * | 风险分 | 等级 | 处理策略 |
 * |--------|------|---------|
 * | 0~29   | low      | 正常游戏 |
 * | 30~49  | medium   | 正常，服务端标记"关注" |
 * | 50~79  | high     | 允许但延迟发放奖励 |
 * | 80~100 | critical | 降级为"仅浏览"模式 |
 */
export function evaluateRisk(report: DeviceSecurityReport): RiskAssessment {
  let score = 0

  if (report.emulatorCheck.isEmulator) {
    score += report.emulatorCheck.riskScore
  } else if (report.emulatorCheck.riskScore > 0) {
    // 部分信号（未达 isEmulator 阈值）按 75% 折算
    score += Math.floor(report.emulatorCheck.riskScore * 0.75)
  }

  if (report.rootCheck.isRooted) {
    score += report.rootCheck.riskScore
  }

  if (report.timeSync.isManipulated) {
    score += 20
  }

  if (report.locationProof.mockDetected) {
    score += 30
  }

  score = Math.min(100, score)

  let level: DeviceRiskLevel = 'low'
  if (score >= 80) level = 'critical'
  else if (score >= 50) level = 'high'
  else if (score >= 30) level = 'medium'

  return { level, score, report }
}
