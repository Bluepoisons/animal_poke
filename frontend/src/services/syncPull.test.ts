import { describe, it, expect, beforeEach } from 'vitest'
import { loadPullCursor, savePullCursor } from './syncPull'

describe('syncPull cursor', () => {
  beforeEach(() => localStorage.clear())
  it('does not store zero cursor overwrite from empty', () => {
    savePullCursor(120)
    expect(loadPullCursor()).toBe(120)
    savePullCursor(0)
    expect(loadPullCursor()).toBe(120)
  })
})
