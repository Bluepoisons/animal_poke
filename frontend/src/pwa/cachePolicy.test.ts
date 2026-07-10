import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))

describe('PWA cache policy', () => {
  it('does not NetworkFirst-cache all /api/ routes', () => {
    const cfg = readFileSync(resolve(here, '../../vite.config.ts'), 'utf8')
    expect(cfg.includes('api-cache')).toBe(false)
    expect(cfg).toMatch(/禁止缓存鉴权/)
  })
})
