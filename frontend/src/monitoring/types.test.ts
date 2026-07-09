import { describe, it, expect } from 'vitest'
import {
  createAlert,
  acknowledgeAlert,
  isInCooldown,
  shouldTriggerRule,
  sortAlertsBySeverity,
  getResponseTime,
  DEFAULT_ALERT_RULES,
  ALERT_LEVEL_PRIORITY,
} from './types'

describe('monitoring', () => {
  it('creates alert with correct fields', () => {
    const alert = createAlert('warning', 'performance', 'High CPU', 'CPU at 90%', 'server-1')
    expect(alert.level).toBe('warning')
    expect(alert.acknowledged).toBe(false)
    expect(alert.id).toBeTruthy()
  })

  it('acknowledges alert', () => {
    const alert = createAlert('critical', 'error', 'Crash', 'App crashed', 'app')
    const acked = acknowledgeAlert(alert, 'admin')
    expect(acked.acknowledged).toBe(true)
    expect(acked.acknowledgedBy).toBe('admin')
  })

  it('checks cooldown', () => {
    const now = Date.now()
    // cooldown 1 second
    expect(isInCooldown(now - 500, 1, now)).toBe(true) // 500ms < 1s
    expect(isInCooldown(now - 2000, 1, now)).toBe(false) // 2s > 1s
  })

  it('triggers rule when threshold exceeded', () => {
    const rule = DEFAULT_ALERT_RULES.find(r => r.id === 'rule-crash-rate')!
    expect(shouldTriggerRule(rule, 0.8, 0)).toBe(true) // 0.8 > 0.5
  })

  it('does not trigger when below threshold', () => {
    const rule = DEFAULT_ALERT_RULES.find(r => r.id === 'rule-crash-rate')!
    expect(shouldTriggerRule(rule, 0.3, 0)).toBe(false) // 0.3 < 0.5
  })

  it('does not trigger during cooldown', () => {
    const rule = DEFAULT_ALERT_RULES.find(r => r.id === 'rule-crash-rate')!
    const recentTime = Date.now() - 10000 // 10s ago
    expect(shouldTriggerRule(rule, 0.8, recentTime)).toBe(false) // cooldown 600s
  })

  it('does not trigger disabled rule', () => {
    const rule = { ...DEFAULT_ALERT_RULES[0], enabled: false }
    expect(shouldTriggerRule(rule, 999, 0)).toBe(false)
  })

  it('sorts alerts by severity then timestamp', () => {
    const alerts = [
      createAlert('info', 'performance', 'A', 'a', 's1'),
      createAlert('fatal', 'error', 'B', 'b', 's2'),
      createAlert('warning', 'security', 'C', 'c', 's3'),
    ]
    const sorted = sortAlertsBySeverity(alerts)
    expect(sorted[0].level).toBe('fatal')
    expect(sorted[sorted.length - 1].level).toBe('info')
  })

  it('calculates response time', () => {
    const alert = createAlert('critical', 'error', 'Test', 'test', 's')
    const acked = acknowledgeAlert(alert, 'admin')
    const responseTime = getResponseTime(acked)
    expect(responseTime).not.toBeNull()
    expect(responseTime!).toBeGreaterThanOrEqual(0)
  })

  it('returns null response time for unacknowledged alert', () => {
    const alert = createAlert('info', 'performance', 'Test', 'test', 's')
    expect(getResponseTime(alert)).toBeNull()
  })

  it('has 5 default alert rules', () => {
    expect(DEFAULT_ALERT_RULES.length).toBe(5)
  })
})
