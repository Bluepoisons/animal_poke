import { describe, it, expect } from 'vitest'
import { FEATURE_FLAGS } from './featureFlags'

describe('featureFlags', () => {
  it('hides unfinished social modules by default', () => {
    expect(FEATURE_FLAGS.dispatch).toBe(false)
    expect(FEATURE_FLAGS.pvp).toBe(false)
  })
})
