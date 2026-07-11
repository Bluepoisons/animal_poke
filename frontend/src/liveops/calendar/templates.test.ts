import { describe, expect, it } from 'vitest'
import {
  TEMPLATES,
  buildNinetyDayCalendar,
  canStack,
  missDoesNotLockCore,
  templateById,
} from './templates'

describe('AP-105 live events calendar', () => {
  it('exposes at least three configurable templates', () => {
    expect(TEMPLATES.length).toBeGreaterThanOrEqual(3)
    for (const t of TEMPLATES) {
      expect(t.safety.nightOutdoorRequired).toBe(false)
      expect(t.safety.crossCityChase).toBe(false)
      expect(missDoesNotLockCore(t)).toBe(true)
      expect(t.rollback.length).toBeGreaterThan(0)
      expect(t.welfareReview.length).toBeGreaterThan(0)
    }
  })

  it('builds 90-day coverage without exceeding day 89 start', () => {
    const cal = buildNinetyDayCalendar()
    expect(cal.length).toBeGreaterThan(5)
    expect(cal.every((s) => s.dayOffset >= 0 && s.dayOffset < 90)).toBe(true)
    expect(cal.every((s) => templateById(s.templateId))).toBe(true)
    const last = cal[cal.length - 1]
    const lastTpl = templateById(last.templateId)!
    expect(last.dayOffset + lastTpl.durationDays).toBeLessThanOrEqual(95)
  })

  it('only allows welfare soft-stack', () => {
    const obs = templateById('tpl.observation_week')!
    const welfare = templateById('tpl.welfare')!
    const research = templateById('tpl.city_research')!
    expect(canStack(obs, research)).toBe(false)
    expect(canStack(obs, welfare)).toBe(true)
  })
})
