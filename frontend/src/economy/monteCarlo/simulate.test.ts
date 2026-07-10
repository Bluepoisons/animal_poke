import { describe, it, expect } from 'vitest'
import { createMulberry32 } from './rng'
import { runSimulation, runSuite, suiteOk, assertNoFreeInfiniteGoldPath } from './simulate'
import { ARCHETYPES } from './config'

describe('rng deterministic', () => {
  it('same seed → same sequence', () => {
    const a = createMulberry32(42)
    const b = createMulberry32(42)
    expect([a(), a(), a()]).toEqual([b(), b(), b()])
  })
})

describe('monte carlo invariants', () => {
  it('30-day newbie run has no negative resources', () => {
    const r = runSimulation({ days: 30, seed: 1, archetype: 'newbie' })
    expect(r.finalGold).toBeGreaterThanOrEqual(0)
    expect(r.finalStamina).toBeGreaterThanOrEqual(0)
    expect(r.ok).toBe(true)
    expect(r.breaches).toEqual([])
  })

  it('90-day all archetypes suite passes gate', () => {
    const results = runSuite([7, 13, 99], [30, 90])
    const { ok, failures } = suiteOk(results)
    if (!ok) {
      // eslint-disable-next-line no-console
      console.error(failures.map((f) => ({ a: f.archetype, d: f.days, b: f.breaches })))
    }
    expect(ok).toBe(true)
    expect(results.length).toBe(2 * 4 * 3) // horizons * archetypes * seeds
  })

  it('free infinite gold path is detected by probe', () => {
    const breaches = assertNoFreeInfiniteGoldPath()
    expect(breaches.some((b) => b.id === 'no_free_infinite_gold')).toBe(true)
  })

  it('all archetypes defined', () => {
    expect(Object.keys(ARCHETYPES).sort()).toEqual(
      ['casual', 'core', 'newbie', 'returning'].sort(),
    )
  })
})
