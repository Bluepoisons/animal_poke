import { describe, expect, it, vi, afterEach } from 'vitest'
import { isAuthMockOAuthEnabled, listProductionAuthProviders } from './authProviders'

describe('authProviders (AP-063)', () => {
  afterEach(() => {
    vi.unstubAllEnvs()
  })

  it('disables mock oauth in production mode even if flag is set', () => {
    vi.stubEnv('MODE', 'production')
    vi.stubEnv('VITE_AUTH_MOCK_OAUTH', '1')
    expect(isAuthMockOAuthEnabled()).toBe(false)
  })

  it('disables mock oauth in staging', () => {
    vi.stubEnv('MODE', 'staging')
    vi.stubEnv('VITE_AUTH_MOCK_OAUTH', 'true')
    expect(isAuthMockOAuthEnabled()).toBe(false)
  })

  it('requires explicit flag in development', () => {
    vi.stubEnv('MODE', 'development')
    vi.stubEnv('VITE_AUTH_MOCK_OAUTH', '')
    expect(isAuthMockOAuthEnabled()).toBe(false)

    vi.stubEnv('VITE_AUTH_MOCK_OAUTH', '1')
    expect(isAuthMockOAuthEnabled()).toBe(true)
  })

  it('allows mock in vitest/test mode for fixtures', () => {
    vi.stubEnv('MODE', 'test')
    vi.stubEnv('VITE_AUTH_MOCK_OAUTH', '')
    expect(isAuthMockOAuthEnabled()).toBe(true)
  })

  it('lists only email for production providers', () => {
    expect(listProductionAuthProviders()).toEqual(['email'])
  })
})
