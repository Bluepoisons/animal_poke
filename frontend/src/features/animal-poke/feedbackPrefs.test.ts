import { describe, it, expect, beforeEach } from 'vitest'
import { loadFeedbackPrefs, saveFeedbackPrefs, announceRareReveal } from './feedbackPrefs'

describe('feedbackPrefs', () => {
  beforeEach(() => localStorage.clear())
  it('defaults enabled', () => {
    expect(loadFeedbackPrefs().soundEnabled).toBe(true)
  })
  it('persists toggles', () => {
    saveFeedbackPrefs({ soundEnabled: false, hapticsEnabled: false, rareRevealEnabled: true })
    expect(loadFeedbackPrefs().soundEnabled).toBe(false)
  })
  it('announces rare only when enabled', () => {
    expect(announceRareReveal('legendary')).toContain('稀有')
    saveFeedbackPrefs({ soundEnabled: true, hapticsEnabled: false, rareRevealEnabled: false })
    expect(announceRareReveal('legendary')).toBeNull()
  })
})
