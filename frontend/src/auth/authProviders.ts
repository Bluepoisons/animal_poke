/**
 * Auth provider visibility (AP-063).
 * mock_oauth is development/test only; production builds never surface it.
 */

/**
 * Whether Mock OAuth UI/API helpers may be used in this build.
 * - production / staging: always false
 * - development / test: true only when VITE_AUTH_MOCK_OAUTH is explicitly 1/true
 *   (or MODE is test, for fixture-driven tests)
 */
export function isAuthMockOAuthEnabled(): boolean {
  const mode = String(import.meta.env.MODE || 'development').toLowerCase()
  if (mode === 'production' || mode === 'staging') {
    return false
  }
  const flag = String(import.meta.env.VITE_AUTH_MOCK_OAUTH || '').toLowerCase()
  if (['1', 'true', 'yes', 'on'].includes(flag)) {
    return true
  }
  // Vitest default MODE is 'test' — allow mock for unit tests without extra env.
  return mode === 'test'
}

/** Production-safe list of user-facing providers. */
export function listProductionAuthProviders(): Array<'email'> {
  return ['email']
}
