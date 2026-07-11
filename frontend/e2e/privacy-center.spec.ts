import { test, expect } from '@playwright/test'
import {
  createApiCallLog,
  installApiMocks,
  installBrowserMocks,
} from './helpers/mocks'

/**
 * AP-068 Privacy UX center on production AnimalPokeApp settings path.
 */

test.describe('AP-068 privacy center', () => {
  test('settings hash shows privacy scopes and distinct delete actions', async ({ page }) => {
    const log = createApiCallLog()
    await installBrowserMocks(page)
    await installApiMocks(page, log)

    // Grant consent in localStorage before load so gate does not block settings
    await page.addInitScript(() => {
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
    })

    let exportHits = 0
    let deleteHits = 0
    await page.route(/\/api\/v1\/privacy\/export/, async (route) => {
      exportHits += 1
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          request_id: 'e2e-export-1',
          status: 'completed',
          data: { animals: [], consent: { version: 'v1' } },
        }),
      })
    })
    await page.route(/\/api\/v1\/privacy\/delete/, async (route) => {
      deleteHits += 1
      const body = route.request().postDataJSON() as { scope?: string }
      if (body?.scope === 'account') {
        await route.fulfill({
          status: 403,
          contentType: 'application/json',
          body: JSON.stringify({ error: 'reauth_required', message: 'reauth required' }),
        })
        return
      }
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ request_id: 'e2e-del-1', status: 'completed', scope: 'device' }),
      })
    })

    await page.goto('/#settings')
    await expect(page.getByTestId('privacy-center')).toBeVisible({ timeout: 15_000 })
    await expect(page.getByTestId('privacy-scope-photo')).toBeVisible()
    await expect(page.getByTestId('privacy-scope-analytics')).toBeVisible()
    await expect(page.getByTestId('privacy-export-server')).toBeVisible()
    await expect(page.getByTestId('privacy-delete-local')).toBeVisible()
    await expect(page.getByTestId('privacy-delete-device')).toBeVisible()
    await expect(page.getByTestId('privacy-delete-account')).toBeVisible()

    // Clipboard may be restricted in headless; grant if available
    await page.context().grantPermissions(['clipboard-read', 'clipboard-write']).catch(() => {})

    page.once('dialog', (d) => d.accept())
    // Server export — should hit API
    await page.getByTestId('privacy-export-server').click()
    await expect.poll(() => exportHits).toBeGreaterThan(0)

    // Failed account delete without password should not claim success
    await page.getByTestId('privacy-delete-account').click()
    await expect(page.getByText(/密码|password|reauth/i)).toBeVisible({ timeout: 5_000 })
    expect(deleteHits).toBe(0)
  })
})
