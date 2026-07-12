import { describe, it, expect } from 'vitest'
import { chapter1Pack, resolveChapter1, validateChapter1 } from './alleyEcho'

describe('AP-125 chapter 1 alley echo', () => {
  it('validates pack', () => {
    expect(validateChapter1()).toEqual([])
  })

  it('home mode uses hotline route', () => {
    const r = resolveChapter1({ homeMode: true, noCamera: true, badWeather: false })
    expect(r.route).toBe('home_hotline')
    expect(r.sequenceIds.length).toBe(3)
  })

  it('has three perspectives and choice', () => {
    expect(chapter1Pack.perspectives).toHaveLength(3)
  })
})
