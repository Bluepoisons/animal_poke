import { describe, expect, it } from 'vitest'
import { ensureFictionMeta, layerLabel } from './fictionLabel'

describe('fictionLabel AP-131', () => {
  it('labels layers', () => {
    expect(layerLabel('fictional_vignette')).toContain('虚构')
    expect(layerLabel('authored_canon')).toContain('正典')
    expect(layerLabel('fact')).toContain('事实')
  })
  it('defaults value narrative to fiction', () => {
    const m = ensureFictionMeta({ narrative: 'hello' })
    expect(m.fiction).toBe(true)
    expect(m.layer).toBe('fictional_vignette')
  })
})
