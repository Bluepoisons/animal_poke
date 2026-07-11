import { describe, expect, it } from 'vitest'
import {
  chapter2Pack,
  completionUnlocks,
  resolveChapter2Route,
  type SafetyContext,
} from './alongRiverSleepless'

const base: SafetyContext = {
  hour: 14,
  weather: 'clear',
  isMinor: false,
  homeMode: false,
  lowMobility: false,
  ch1Choices: [],
}

describe('AP-126 chapter 2 《沿河不眠》', () => {
  it('pack has dual non-authoritarian conflict options', () => {
    expect(chapter2Pack.debateBeats.length).toBeGreaterThanOrEqual(2)
    expect(chapter2Pack.sequences.some((s) => s.id === 'ch02.conflict')).toBe(true)
    const conflict = chapter2Pack.sequences.find((s) => s.id === 'ch02.conflict')!
    const choiceSeg = conflict.segments.find((s) => s.choice)
    expect(choiceSeg?.choice?.options.length).toBe(2)
  })

  it('never requires outdoor at night, storm, minor, or home mode', () => {
    const cases: SafetyContext[] = [
      { ...base, hour: 22 },
      { ...base, weather: 'storm' },
      { ...base, isMinor: true },
      { ...base, homeMode: true },
      { ...base, lowMobility: true },
    ]
    for (const ctx of cases) {
      const r = resolveChapter2Route(ctx, 'n_open')
      expect(r.outdoorRequired).toBe(false)
      expect(['home_desk', 'memory_night']).toContain(r.route)
    }
  })

  it('inherits chapter1 choice into route or attitude', () => {
    const ally = resolveChapter2Route({ ...base, ch1Choices: ['trust_guide'] }, 'n_conflict')
    expect(ally.route).toBe('ally_path')
    const att = resolveChapter2Route({ ...base, ch1Choices: ['protect_habitat'] }, 'n_conflict')
    expect(att.ch1AttitudeMod).toMatch(/志愿者/)
  })

  it('completion unlocks memory layer and ch3 rumor', () => {
    expect(completionUnlocks('n_resolve')).toEqual(
      expect.arrayContaining(['layer.river_memory', 'rumor.ch03.wharf']),
    )
  })

  it('debate beats forbid real animal harm framing', () => {
    for (const b of chapter2Pack.debateBeats) {
      expect(b.noHarmNote).toMatch(/不/)
    }
  })
})
