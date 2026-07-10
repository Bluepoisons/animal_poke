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
 */
export function evaluateRisk(report: DeviceSecurityReport): RiskAssessment {
  let score = 0

  if (report.emulatorCheck.isEmulator) {
    score += report.emulatorCheck.riskScore
  } else if (report.emulatorCheck.riskScore > 0) {
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

/**
 * 提交安全报告到后端 /api/v1/security/report。
 * token 可选；失败时返回 null 且不默认安全。
 */
export async function submitSecurityReport(
  report: DeviceSecurityReport,
  token?: string,
  baseUrl = '',
): Promise<{ risk_score: number; safe: boolean } | null> {
  try {
    const nonce = crypto.randomUUID?.() ?? `${Date.now()}-${Math.random()}`
    const headers: Record<string, string> = { 'Content-Type': 'application/json' }
    if (token) headers.Authorization = `Bearer ${token}`
    const resp = await fetch(`${baseUrl}/api/v1/security/report`, {
      method: 'POST',
      headers,
      body: JSON.stringify({
        nonce,
        payload: {
          client_skew_ms: report.timeSync.offset,
          debugger: report.emulatorCheck.isEmulator,
          rooted: report.rootCheck.isRooted,
          collected_at: report.collectedAt,
        },
      }),
    })
    if (!resp.ok) return null
    return (await resp.json()) as { risk_score: number; safe: boolean }
  } catch {
    return null
  }
}
