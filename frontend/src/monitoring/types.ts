/** 监控告警 — 异常自动告警 < 1min */

export type AlertLevel = 'info' | 'warning' | 'critical' | 'fatal'
export type AlertCategory = 'performance' | 'error' | 'availability' | 'security' | 'capacity'

export interface Alert {
  id: string
  level: AlertLevel
  category: AlertCategory
  title: string
  message: string
  source: string
  timestamp: number
  acknowledged: boolean
  acknowledgedBy: string | null
  acknowledgedAt: number | null
  metadata: Record<string, unknown>
}

export interface AlertRule {
  id: string
  name: string
  category: AlertCategory
  metric: string
  threshold: number
  comparison: 'gt' | 'lt' | 'eq'
  windowSeconds: number
  cooldownSeconds: number
  enabled: boolean
}

export interface AlertChannel {
  type: 'webhook' | 'email' | 'sms' | 'dashboard'
  target: string
  enabled: boolean
}

/** 告警等级颜色映射 */
export const ALERT_LEVEL_COLORS: Record<AlertLevel, string> = {
  info: '#3498db',
  warning: '#f39c12',
  critical: '#e74c3c',
  fatal: '#8b0000',
}

/** 告警等级优先级 */
export const ALERT_LEVEL_PRIORITY: Record<AlertLevel, number> = {
  info: 1,
  warning: 2,
  critical: 3,
  fatal: 4,
}

/** 默认告警规则 */
export const DEFAULT_ALERT_RULES: AlertRule[] = [
  {
    id: 'rule-crash-rate',
    name: '崩溃率超标',
    category: 'error',
    metric: 'crash_rate_percent',
    threshold: 0.5,
    comparison: 'gt',
    windowSeconds: 300,
    cooldownSeconds: 600,
    enabled: true,
  },
  {
    id: 'rule-api-latency',
    name: 'API 延迟过高',
    category: 'performance',
    metric: 'p99_latency_ms',
    threshold: 500,
    comparison: 'gt',
    windowSeconds: 60,
    cooldownSeconds: 300,
    enabled: true,
  },
  {
    id: 'rule-server-down',
    name: '服务器不可用',
    category: 'availability',
    metric: 'health_check_failures',
    threshold: 3,
    comparison: 'gt',
    windowSeconds: 60,
    cooldownSeconds: 60,
    enabled: true,
  },
  {
    id: 'rule-cheat-spike',
    name: '作弊检测激增',
    category: 'security',
    metric: 'cheat_detections_per_min',
    threshold: 10,
    comparison: 'gt',
    windowSeconds: 60,
    cooldownSeconds: 300,
    enabled: true,
  },
  {
    id: 'rule-cpu-high',
    name: 'CPU 使用率过高',
    category: 'capacity',
    metric: 'cpu_usage_percent',
    threshold: 85,
    comparison: 'gt',
    windowSeconds: 120,
    cooldownSeconds: 300,
    enabled: true,
  },
]

/** 创建告警 */
export function createAlert(
  level: AlertLevel,
  category: AlertCategory,
  title: string,
  message: string,
  source: string,
  metadata: Record<string, unknown> = {},
): Alert {
  const now = Date.now()
  return {
    id: `alert-${now}-${Math.random().toString(36).slice(2, 8)}`,
    level,
    category,
    title,
    message,
    source,
    timestamp: now,
    acknowledged: false,
    acknowledgedBy: null,
    acknowledgedAt: null,
    metadata,
  }
}

/** 确认告警 */
export function acknowledgeAlert(
  alert: Alert,
  userId: string,
): Alert {
  return {
    ...alert,
    acknowledged: true,
    acknowledgedBy: userId,
    acknowledgedAt: Date.now(),
  }
}

/** 检查告警是否在冷却期 */
export function isInCooldown(
  lastAlertTime: number,
  cooldownSeconds: number,
  now: number = Date.now(),
): boolean {
  return (now - lastAlertTime) < cooldownSeconds * 1000
}

/** 检查规则是否触发 */
export function shouldTriggerRule(
  rule: AlertRule,
  currentValue: number,
  lastTriggerTime: number,
  now: number = Date.now(),
): boolean {
  if (!rule.enabled) return false
  if (isInCooldown(lastTriggerTime, rule.cooldownSeconds, now)) return false

  switch (rule.comparison) {
    case 'gt': return currentValue > rule.threshold
    case 'lt': return currentValue < rule.threshold
    case 'eq': return currentValue === rule.threshold
    default: return false
  }
}

/** 按严重程度排序告警 */
export function sortAlertsBySeverity(alerts: Alert[]): Alert[] {
  return [...alerts].sort((a, b) =>
    ALERT_LEVEL_PRIORITY[b.level] - ALERT_LEVEL_PRIORITY[a.level] ||
    b.timestamp - a.timestamp,
  )
}

/** 计算告警响应时间（从触发到确认） */
export function getResponseTime(alert: Alert): number | null {
  if (!alert.acknowledged || !alert.acknowledgedAt) return null
  return alert.acknowledgedAt - alert.timestamp
}
