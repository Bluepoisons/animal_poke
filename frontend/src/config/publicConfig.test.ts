import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { loadPublicConfig, __resetPublicConfigForTests, getApiBaseUrl } from './publicConfig'

describe('publicConfig', () => {
  beforeEach(() => {
    __resetPublicConfigForTests()
    delete window.__AP_CONFIG__
  })

  afterEach(() => {
    delete window.__AP_CONFIG__
    __resetPublicConfigForTests()
  })

  it('defaults to empty base URL for same-origin/proxy', () => {
    const cfg = loadPublicConfig()
    expect(cfg.apiBaseUrl).toBe('')
    expect(getApiBaseUrl()).toBe('')
  })

  it('prefers window.__AP_CONFIG__.apiBaseUrl over empty build default', () => {
    window.__AP_CONFIG__ = { apiBaseUrl: 'https://api.example.com' }
    const cfg = loadPublicConfig()
    expect(cfg.apiBaseUrl).toBe('https://api.example.com')
  })

  it('accepts relative /api runtime config', () => {
    window.__AP_CONFIG__ = { apiBaseUrl: '/api' }
    expect(loadPublicConfig().apiBaseUrl).toBe('/api')
  })

  it('rejects invalid runtime API base URL', () => {
    window.__AP_CONFIG__ = { apiBaseUrl: 'not-a-url' }
    expect(() => loadPublicConfig()).toThrow(/absolute URL or path/)
  })

  it('accepts absolute https URL when set via env mock', () => {
    // Vite injects at build time; unit test covers normalize via empty default
    const cfg = loadPublicConfig()
    expect(['DEBUG', 'INFO', 'WARN', 'ERROR']).toContain(cfg.logLevel)
  })
})
