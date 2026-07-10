import { describe, it, expect, beforeEach } from 'vitest'
import { advanceOnboarding, loadOnboarding, resetOnboarding, skipOnboarding } from './onboarding'

describe('onboarding', () => {
  beforeEach(() => {
    localStorage.clear()
    resetOnboarding()
  })
  it('advances steps until done', () => {
    let s = loadOnboarding()
    expect(s.step).toBe('welcome')
    for (let i = 0; i < 6; i++) s = advanceOnboarding()
    expect(s.step).toBe('done')
  })
  it('can skip', () => {
    const s = skipOnboarding()
    expect(s.skipped).toBe(true)
    expect(s.step).toBe('done')
  })
})
