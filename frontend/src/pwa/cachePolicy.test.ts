import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

describe('PWA cache policy', () => {
  it('does not NetworkFirst-cache all /api/ routes', () => {
    const cfg = readFileSync(resolve(__dirname, '../../vite.config.ts'), 'utf8')
    expect(cfg.includes('api-cache')).toBe(false)
    expect(cfg).toMatch(/禁止缓存鉴权/)
  })
})
