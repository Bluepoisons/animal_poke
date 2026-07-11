import { describe, expect, it } from 'vitest'
import { failForwardAdvancesStory, reasonFromCaptureFailure } from './failForward'

describe('failForward AP-120', () => {
  it('maps failures', () => {
    expect(reasonFromCaptureFailure({ offline: true })).toBe('offline')
    expect(reasonFromCaptureFailure({ weatherBlock: true })).toBe('weather')
    expect(reasonFromCaptureFailure({ noCamera: true })).toBe('permission')
    expect(reasonFromCaptureFailure({})).toBe('miss')
  })
  it('always can advance', () => {
    expect(failForwardAdvancesStory(1)).toBe(true)
    expect(failForwardAdvancesStory(3)).toBe(true)
  })
})
