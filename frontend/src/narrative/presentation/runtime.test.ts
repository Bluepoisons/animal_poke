import { describe, expect, it, vi } from 'vitest'
import { PresentationRuntime, segmentFallbackText } from './runtime'
import { sampleRiverNightSequence } from './sampleSequence'
import type { PresentationCheckpoint, PresentationSequence } from './types'

function seq(): PresentationSequence {
  return structuredClone(sampleRiverNightSequence)
}

describe('PresentationRuntime AP-122', () => {
  it('starts at entry and builds skip summary without losing plot', () => {
    const rt = new PresentationRuntime(seq())
    const s0 = rt.start(false)
    expect(s0.segment?.id).toBe('s1')
    expect(s0.status).toBe('playing')
    rt.skip()
    rt.skip()
    const s = rt.snapshot()
    expect(s.log.length).toBeGreaterThanOrEqual(2)
    expect(rt.fullSummary()).toContain('河堤')
  })

  it('blocks skip past critical choice until chosen; choice is idempotent', () => {
    const onChoice = vi.fn()
    const rt = new PresentationRuntime(seq(), { onChoice })
    rt.start(false)
    // jump to choice segment
    while (rt.current()?.id !== 's5') rt.skip()
    expect(rt.snapshot().status).toBe('awaiting_choice')
    const blocked = rt.skip()
    expect(blocked.segment?.id).toBe('s5')
    expect(onChoice).not.toHaveBeenCalled()

    rt.choose('c_light')
    expect(onChoice).toHaveBeenCalledTimes(1)
    expect(rt.snapshot().segment?.id).toBe('s6a')

    // rewind and re-choose same — no second submit
    rt.rewind()
    rt.choose('c_light')
    expect(onChoice).toHaveBeenCalledTimes(1)
    // different choice ignored once locked
    rt.choose('c_water')
    expect(onChoice).toHaveBeenCalledTimes(1)
  })

  it('restores checkpoint and does not reset choice', () => {
    let saved: PresentationCheckpoint | null = null
    const rt1 = new PresentationRuntime(seq(), {
      save: (cp) => {
        saved = structuredClone(cp)
      },
      load: () => saved,
    })
    rt1.start(false)
    while (rt1.current()?.id !== 's5') rt1.skip()
    rt1.choose('c_water')
    expect(saved).not.toBeNull()
    expect(saved!.choices?.['s5']).toBe('c_water')

    const rt2 = new PresentationRuntime(seq(), {
      save: (cp) => {
        saved = cp
      },
      load: () => saved,
    })
    const s = rt2.start(true)
    expect(s.segment?.id).toBe('s6b')
    expect(s.log.some((l) => l.includes('水声'))).toBe(true)
  })

  it('degrades to transcript/text when assets fail or muted', () => {
    const rt = new PresentationRuntime(seq(), { muted: true })
    rt.start(false)
    while (rt.current()?.kind !== 'voice_note') rt.skip()
    const seg = rt.current()!
    rt.markAssetFailed(seg.voiceNote!.audioUrl!)
    const text = segmentFallbackText(seg, (u) => rt.isDegraded(u))
    expect(text).toContain('水流')
    expect(rt.snapshot().failedAssets.length).toBe(1)
  })

  it('pause freezes status and resume continues', () => {
    let t = 1_000
    const rt = new PresentationRuntime(seq(), { now: () => t })
    rt.start(false)
    rt.pause()
    expect(rt.snapshot().status).toBe('paused')
    t += 5_000
    rt.resume()
    expect(rt.snapshot().status).toBe('playing')
  })

  it('auto tick skips non-choice under time budget', () => {
    const rt = new PresentationRuntime(seq(), { auto: true, reducedMotion: false })
    rt.start(false)
    const before = rt.snapshot().index
    rt.tickAuto(4000)
    expect(rt.snapshot().index).toBeGreaterThan(before)
  })
})
