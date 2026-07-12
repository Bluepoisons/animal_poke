import { test, expect } from '@playwright/test'
import { installBrowserMocks } from './helpers/mocks'

test.describe('store smoke loop', () => {
  test('daily check-in rewards persist across refresh', async ({ page }) => {
    await installBrowserMocks(page)
    await page.addInitScript(() => {
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
    })
    await page.route(/\/api\/v1\//, async (route) => {
      const path = new URL(route.request().url()).pathname
      if (path.includes('/auth/device')) {
        return route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            token: 'e2e-test-token',
            expires_at: new Date(Date.now() + 3600_000).toISOString(),
          }),
        })
      }
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ ok: true }) })
    })

    await page.goto('/#store')
    const checkIn = page.getByRole('button', { name: /7 日签到轨道/ })
    await expect(checkIn).toBeEnabled({ timeout: 15_000 })
    await checkIn.click()
    await expect(page.getByRole('status').filter({ hasText: /签到成功/ })).toBeVisible()

    await page.reload()
    await expect(page.getByRole('button', { name: /7 日签到轨道/ })).toBeDisabled()
    await expect(page.getByText(/今天的贴纸已领取/)).toBeVisible()
  })
})
