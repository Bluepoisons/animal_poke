import { describe, expect, it } from 'vitest'
import {
  activeCaptions,
  defaultPlaybackState,
  mayAutoplay,
  onAppBackground,
  pickClipLocale,
  resolvePlayback,
  type AudioClipRef,
} from './policy'

const baseClip = (over: Partial<AudioClipRef> = {}): AudioClipRef => ({
  id: 'c1',
  bus: 'voice',
  url: '/a.opus',
  locale: 'zh',
  version: 1,
  licenseId: 'lic.self.guide_open_v1',
  captions: [
    { id: '1', speaker: '向导', text: '小心潮位', startMs: 0, endMs: 1200 },
    { id: '2', sfxDescription: '闸门低鸣', text: '', startMs: 1200, endMs: 2000 },
  ],
  transcript: '向导：小心潮位。[闸门低鸣]',
  ...over,
})

describe('AP-123 audio policy', () => {
  it('never autoplays voice or startling clips', () => {
    const st = defaultPlaybackState()
    expect(mayAutoplay(baseClip(), st)).toBe(false)
    expect(mayAutoplay(baseClip({ bus: 'ambience', startling: true }), st)).toBe(false)
    expect(mayAutoplay(baseClip({ bus: 'ambience' }), st)).toBe(true)
  })

  it('falls back to captions when muted or asset failed', () => {
    const st = { ...defaultPlaybackState(), muted: true }
    const r = resolvePlayback(baseClip(), st)
    expect(r.mode).toBe('captions_only')
    expect(r.transcript).toContain('潮位')
    expect(resolvePlayback(baseClip(), defaultPlaybackState(), true).mode).toBe('captions_only')
  })

  it('stops on background and does not imply autoplay resume', () => {
    const bg = onAppBackground(defaultPlaybackState())
    expect(bg.backgrounded).toBe(true)
    expect(mayAutoplay(baseClip({ bus: 'music' }), bg)).toBe(false)
  })

  it('locale fallback prefers preferred list then transcript-only locale', () => {
    const clips = [
      baseClip({ locale: 'en', url: undefined, transcript: 'EN text' }),
      baseClip({ locale: 'zh', url: '/zh.opus' }),
    ]
    expect(pickClipLocale(clips, ['zh', 'en'])?.locale).toBe('zh')
    expect(pickClipLocale(clips, ['ja', 'en'])?.locale).toBe('en')
  })

  it('active captions include sfx descriptions', () => {
    const cues = baseClip().captions
    expect(activeCaptions(cues, 1500)[0]?.sfxDescription).toBe('闸门低鸣')
  })
})
