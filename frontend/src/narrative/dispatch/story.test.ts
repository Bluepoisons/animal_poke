import { describe, expect, it } from 'vitest'
import { listDestinations, planThreeDayChains, runDispatchStory } from './story'

describe('AP-104 dispatch narrative', () => {
  it('is deterministic for seed+day+destination', () => {
    const a = runDispatchStory({
      seed: 'dev1',
      dayKey: '2026-07-11',
      destination: 'riverside',
      recentChains: [],
    })
    const b = runDispatchStory({
      seed: 'dev1',
      dayKey: '2026-07-11',
      destination: 'riverside',
      recentChains: [],
    })
    expect(a.claimToken).toBe(b.claimToken)
    expect(a.beats.join()).toBe(b.beats.join())
    expect(a.beats.join()).not.toMatch(/伤|死|救治|倒计时恐吓/)
  })

  it('claim token stable for offline reclaim', () => {
    const r = runDispatchStory({
      seed: 'x',
      dayKey: '2026-07-11',
      destination: 'library_yard',
      recentChains: [],
    })
    expect(r.claimToken.startsWith('dispatch:2026-07-11:')).toBe(true)
  })

  it('can avoid repeating main chain across three days with varied destinations', () => {
    const dests = listDestinations()
    const chains = planThreeDayChains('seed-abc', dests)
    expect(new Set(chains).size).toBeGreaterThanOrEqual(2)
  })
})
