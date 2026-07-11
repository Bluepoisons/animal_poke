import { describe, expect, it } from 'vitest'
import { chapterComplete, segmentSkip, surveySample } from './events'

describe('AP-135 narrative analytics events', () => {
  it('scrubs forbidden keys and keeps versioned props', () => {
    const e = chapterComplete('ch02.v1', true)
    expect(e.name).toBe('narrative_chapter_complete')
    expect(e.props.chapter_version).toBe('ch02.v1')
    expect(e.props).not.toHaveProperty('photo')
  })

  it('encodes skip reasons for dashboard split', () => {
    expect(segmentSkip('ch02.v1', 's1', 'confused').props.reason).toBe('confused')
    expect(segmentSkip('ch02.v1', 's1', 'intentional').props.reason).toBe('intentional')
  })

  it('clamps survey scores', () => {
    const s = surveySample('ch02.v1', 2, -1)
    expect(s.props.meaningful_choice).toBe(1)
    expect(s.props.felt_lectured).toBe(0)
  })
})
