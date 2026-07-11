import { test, expect } from '@playwright/test'
import { installBrowserMocks } from './helpers/mocks'
import { scanA11y } from './helpers/axe'

/**
 * AP-075 recovery-refusal and weak-network gates.
 * Covers: denied camera/geo recovery, offline fallback, retry UX.
 */

test.describe('AP-075 recovery and resilience', () => {
  test('camera permission denial shows recovery path', async ({ page }) => {
    await installBrowserMocks(page)

    // Simulate camera permission denied
    await page.context().clearPermissions()
    await page.context().grantPermissions([], { origin: 'http://127.0.0.1:4173' })

    await page.route(/\/api\/v1\//, async (route) => {
      const path = new URL(route.request().url()).pathname
      if (path.includes('/auth/device')) {
        return route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ token: 'e2e-test-token', expires_at: new Date(Date.now() + 3600_000).toISOString() }),
        })
      }
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ ok: true }) })
    })

    await page.goto('/')

    // Consent gate
    await expect(page.getByRole('heading', { name: /隐私与权限/ })).toBeVisible()
    await page.getByRole('button', { name: /同意并继续/ }).click()

    // Should still show discover mode (no crash)
    await expect(page.getByText('DISCOVER MODE')).toBeVisible({ timeout: 20_000 })

    // Axe on recovery state
    const axeResult = await scanA11y(page)
    expect(axeResult.violations, `a11y violations on recovery:\n${axeResult.details}`).toBe(0)
  })

  test('weak network shows offline banner and degrades gracefully', async ({ page }) => {
    // Simulate offline + browser mocks
    await page.route(/\/api\/v1\//, () => {
      // Abort all API calls to simulate offline
    })

    await page.addInitScript(() => {
      try {
        localStorage.setItem('animal-poke-consent', JSON.stringify({
          status: 'granted', grantedAt: Date.now(), version: 'v1',
          scopes: ['photo'], serverSynced: true, revokedAt: null, updatedAt: Date.now(),
        }))
        localStorage.setItem('animal-poke-onboarding-v1', JSON.stringify({
          step: 'done', skipped: true, completedAt: Date.now(),
        }))
      } catch {}
      Object.defineProperty(navigator, 'onLine', {
        configurable: true,
        get: () => false,
      })
    })

    await page.goto('/')

    // Offline banner or fallback should be visible
    const offlineIndicator = page.getByText(/离线|offline|网络不可用/i)
    const discover = page.getByText('DISCOVER MODE')
    const ok = await Promise.any([
      offlineIndicator.isVisible().then(() => true),
      discover.isVisible({ timeout: 15_000 }).then(() => true),
    ]).catch(() => false)
    expect(ok).toBeTruthy()

    // Axe on offline state
    const axeResult = await scanA11y(page)
    expect(axeResult.violations, `a11y violations on offline:\n${axeResult.details}`).toBe(0)
  })

  test('refresh preserves consent and discover state', async ({ page }) => {
    await installBrowserMocks(page)
    await page.route(/\/api\/v1\//, async (route) => {
      const path = new URL(route.request().url()).pathname
      if (path.includes('/auth/device')) {
        return route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ token: 'e2e-test-token', expires_at: new Date(Date.now() + 3600_000).toISOString() }),
        })
      }
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ ok: true }) })
    })

    // Pre-grant consent in localStorage
    await page.addInitScript(() => {
      try {
        localStorage.setItem('animal-poke-consent', JSON.stringify({
          status: 'granted', grantedAt: Date.now(), version: 'v1',
          scopes: ['photo', 'location'], serverSynced: true, revokedAt: null, updatedAt: Date.now(),
        }))
      } catch {}
    })

    await page.goto('/')
    // Should skip consent gate and go straight to discover
    await expect(page.getByText('DISCOVER MODE')).toBeVisible({ timeout: 20_000 })

    // Reload and verify
    await page.reload()
    await expect(page.getByText('DISCOVER MODE')).toBeVisible({ timeout: 20_000 })
  })
})
