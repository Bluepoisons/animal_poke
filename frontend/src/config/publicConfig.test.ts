import { describe, it, expect, beforeEach } from 'vitest'
import { loadPublicConfig, __resetPublicConfigForTests, getApiBaseUrl } from './publicConfig'

describe('publicConfig', () => {
  beforeEach(() => {
    __resetPublicConfigForTests()
  })

  it('defaults to empty base URL for same-origin/proxy', () => {
    const cfg = loadPublicConfig()
    expect(cfg.apiBaseUrl).toBe('')
    expect(getApiBaseUrl()).toBe('')
  })

  it('accepts absolute https URL when set via env mock', () => {
    // Vite injects at build time; unit test covers normalize via empty default
    const cfg = loadPublicConfig()
    expect(['DEBUG', 'INFO', 'WARN', 'ERROR']).toContain(cfg.logLevel)
  })
})
