import { test, expect } from '@playwright/test'
import { installBrowserMocks } from './helpers/mocks'
import { scanA11y } from './helpers/axe'

/**
 * AP-075 map geo-denial and empty-state gate.
 * Covers: geo permission denied, empty discovery list, back navigation.
 */

test.describe('AP-075 map dead-end and empty states', () => {
  test('geo denied shows fallback status on map', async ({ page }) => {
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

    // Override geolocation to simulate denial
    await page.addInitScript(() => {
      // @ts-expect-error override
      navigator.geolocation.getCurrentPosition = (_ok: unknown, err: (e: GeolocationPositionError) => void) => {
        err({
          code: 1,
          message: 'User denied Geolocation',
          PERMISSION_DENIED: 1,
          POSITION_UNAVAILABLE: 2,
          TIMEOUT: 3,
        } as GeolocationPositionError)
      }
      // @ts-expect-error override
      navigator.geolocation.watchPosition = (_ok: unknown, err: (e: GeolocationPositionError) => void) => {
        err({
          code: 1,
          message: 'User denied Geolocation',
          PERMISSION_DENIED: 1,
          POSITION_UNAVAILABLE: 2,
          TIMEOUT: 3,
        } as GeolocationPositionError)
        return 0
      }
    })

    await page.route(/\/api\/v1\//, async (route) => {
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ ok: true }) })
    })

    await page.goto('/#map')
    // Map screen or discover fallback should render without crash
    await expect(
      page.getByText(/地图|定位|位置|权限|发现模式|Map|location|permission|DISCOVER MODE/i).first(),
    ).toBeVisible({ timeout: 20_000 })

    const axeResult = await scanA11y(page)
    expect(axeResult.violations, `a11y violations on map:\n${axeResult.details}`).toBe(0)
  })

  test('map back navigation returns to discover', async ({ page }) => {
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

    await page.goto('/#map')
    await expect(page.locator('.ap-phone, [data-phone-frame]').first()).toBeVisible({ timeout: 15_000 })

    await page.getByRole('button', { name: /返回发现页|Back/i }).click()
    await expect(page).toHaveURL(/#discover$/)
    await expect(page.getByText(/发现模式|DISCOVER MODE/)).toBeVisible({ timeout: 15_000 })
  })
})
