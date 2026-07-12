import { describe, it, expect } from 'vitest'
import {
  ACTIVITY_REWARD_BUDGET,
  HOME_ACTIVITIES,
  d1HomeUnlocks,
  d7HomeUnlocks,
  isHomeModeEquivalent,
  rewardFor,
  validateHomeModeParity,
} from './homeMode'

describe('AP-109 home mode', () => {
  it('validates parity and no camera/location', () => {
    expect(validateHomeModeParity()).toEqual([])
  })

  it('home rewards equal outdoor budget', () => {
    for (const kind of Object.keys(ACTIVITY_REWARD_BUDGET) as (keyof typeof ACTIVITY_REWARD_BUDGET)[]) {
      expect(isHomeModeEquivalent(rewardFor(kind, false), rewardFor(kind, true))).toBe(true)
    }
  })

  it('D1/D7 unlock alternatives exist', () => {
    expect(d1HomeUnlocks().length).toBeGreaterThanOrEqual(3)
    expect(d7HomeUnlocks().length).toBeGreaterThanOrEqual(3)
    expect(HOME_ACTIVITIES.every((a) => !a.needsCamera && !a.needsLocation)).toBe(true)
  })
})
