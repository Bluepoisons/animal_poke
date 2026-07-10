import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))

describe('PWA cache policy', () => {
  const cfg = readFileSync(resolve(here, '../../vite.config.ts'), 'utf8')

  it('does not NetworkFirst-cache all /api/ routes', () => {
    expect(cfg.includes('api-cache')).toBe(false)
    expect(cfg).toMatch(/禁止缓存鉴权/)
  })

  it('uses prompt registration for controlled updates (AP-040)', () => {
    expect(cfg).toMatch(/registerType:\s*'prompt'/)
  })

  it('does not list sensitive API path patterns for runtimeCaching', () => {
    // Ensure no broad /api/ NetworkFirst
    expect(cfg).not.toMatch(/urlPattern:\s*\/\^?\\?\/api/)
    for (const p of ['auth', 'vision', 'value', 'sync', 'privacy']) {
      // Should not appear as a cacheable runtime route pattern
      expect(cfg).not.toMatch(new RegExp(`api/v1/${p}.*NetworkFirst`))
    }
  })
})
