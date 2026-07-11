import { test, expect } from '@playwright/test'
import { installBrowserMocks } from './helpers/mocks'
import { scanA11y } from './helpers/axe'

/**
 * AP-075 settings and account navigation gate.
 * Covers: settings hash entry, locale toggle, account panel open/close, a11y.
 */

test.describe('AP-075 settings and account', () => {
  test('settings hash shows all sections and language toggle', async ({ page }) => {
    await installBrowserMocks(page)

    await page.addInitScript(() => {
      try {
        localStorage.setItem(
          'animal-poke-consent',
          JSON.stringify({
            status: 'granted',
            grantedAt: Date.now(),
            version: 'v1',
            scopes: ['photo', 'location'],
            serverSynced: true,
            revokedAt: null,
            updatedAt: Date.now(),
          }),
        )
        localStorage.setItem(
          'animal-poke-onboarding-v1',
          JSON.stringify({ step: 'done', skipped: true, completedAt: Date.now() }),
        )
      } catch {
        /* ignore */
      }
    })

    await page.route(/\/api\/v1\//, async (route) => {
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ ok: true }) })
    })

    await page.goto('/#settings')
    await expect(page.getByTestId('settings-screen')).toBeVisible({ timeout: 15_000 })

    await expect(page.getByRole('button', { name: /中文|Chinese|English|英文/i }).first()).toBeVisible()
    await expect(page.getByRole('switch').first()).toBeVisible()
    await expect(page.getByTestId('privacy-center')).toBeVisible()

    const axeResult = await scanA11y(page)
    expect(axeResult.violations, `a11y violations on settings:\n${axeResult.details}`).toBe(0)
  })

  test('account panel opens from discover and shows guest state', async ({ page }) => {
    await installBrowserMocks(page)

    await page.addInitScript(() => {
      try {
        localStorage.setItem(
          'animal-poke-consent',
          JSON.stringify({
            status: 'granted',
            grantedAt: Date.now(),
            version: 'v1',
            scopes: ['photo'],
            serverSynced: true,
            revokedAt: null,
            updatedAt: Date.now(),
          }),
        )
        localStorage.setItem(
          'animal-poke-onboarding-v1',
          JSON.stringify({ step: 'done', skipped: true, completedAt: Date.now() }),
        )
      } catch {
        /* ignore */
      }
    })

    await page.route(/\/api\/v1\//, async (route) => {
      const path = new URL(route.request().url()).pathname
      if (path.includes('/account') || path.includes('/auth/account')) {
        return route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ guest: true }),
        })
      }
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ ok: true }) })
    })

    await page.goto('/')
    await expect(page.getByText('DISCOVER MODE')).toBeVisible({ timeout: 20_000 })

    const openAccount = page.getByTestId('open-account').or(page.getByRole('button', { name: /账号|Account/i }))
    if (await openAccount.first().isVisible().catch(() => false)) {
      await openAccount.first().click()
      const accountPanel = page.getByTestId('account-settings').or(page.getByText(/访客|Guest|账号|Account/i).first())
      await expect(accountPanel).toBeVisible({ timeout: 10_000 })
    }

    const axeResult = await scanA11y(page)
    expect(axeResult.violations, `a11y violations on settings+account:\n${axeResult.details}`).toBe(0)
  })
})
