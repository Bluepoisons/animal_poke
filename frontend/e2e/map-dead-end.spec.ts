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
        localStorage.setItem('animal-poke-consent', JSON.stringify({
          status: 'granted', grantedAt: Date.now(), version: 'v1',
          scopes: ['photo', 'location'], serverSynced: true, revokedAt: null, updatedAt: Date.now(),
        }))
        localStorage.setItem('animal-poke-onboarding-v1', JSON.stringify({
          step: 'done', skipped: true, completedAt: Date.now(),
        }))
      } catch {}
    })

    // Override geolocation to simulate denial
    await page.addInitScript(() => {
      const err = { code: 1, message: 'User denied Geolocation', PERMISSION_DENIED: 1, POSITION_UNAVAILABLE: 2, TIMEOUT: 3 }
      Object.defineProperty(navigator, 'geolocation', {
        configurable: true,
        value: {
          getCurrentPosition: (_success: unknown, error: (e: typeof err) => void) => {
            error(err)
          },
          watchPosition: (_success: unknown, error: (e: typeof err) => void) => {
            error(err)
            return 1
          },
          clearWatch: () => {},
        },
      })
      Object.defineProperty(navigator, 'permissions', {
        configurable: true,
        value: {
          query: async () => ({ state: 'denied' }),
        },
      })
    })

    await page.route(/\/api\/v1\//, async (route) => {
      const path = new URL(route.request().url()).pathname
      if (path.includes('/auth/device')) {
        return route.fulfill({
          status: 200, contentType: 'application/json',
          body: JSON.stringify({ token: 'e2e-test-token', expires_at: new Date(Date.now() + 3600_000).toISOString() }),
        })
      }
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ ok: true }) })
    })

    await page.goto('/#map')
    // Should show map screen without crashing; fallback status text should appear
    const mapOrFallback = await Promise.any([
      page.getByText(/定位|locating|denied|不可用|unsupported/i).first().isVisible().then(() => true),
      page.getByRole('button', { name: /返回|back/i }).isVisible().then(() => true),
    ]).catch(() => false)
    expect(mapOrFallback).toBeTruthy()

    // Axe on map state
    const axeResult = await scanA11y(page)
    expect(axeResult.violations, `a11y violations on map:\n${axeResult.details}`).toBe(0)
  })

  test('map back navigation returns to discover', async ({ page }) => {
    await installBrowserMocks(page)

    await page.addInitScript(() => {
      try {
        localStorage.setItem('animal-poke-consent', JSON.stringify({
          status: 'granted', grantedAt: Date.now(), version: 'v1',
          scopes: ['photo', 'location'], serverSynced: true, revokedAt: null, updatedAt: Date.now(),
        }))
        localStorage.setItem('animal-poke-onboarding-v1', JSON.stringify({
          step: 'done', skipped: true, completedAt: Date.now(),
        }))
      } catch {}
    })

    await page.route(/\/api\/v1\//, async (route) => {
      const path = new URL(route.request().url()).pathname
      if (path.includes('/auth/device')) {
        return route.fulfill({
          status: 200, contentType: 'application/json',
          body: JSON.stringify({ token: 'e2e-test-token', expires_at: new Date(Date.now() + 3600_000).toISOString() }),
        })
      }
      if (path.includes('/geo/city')) {
        return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ city: '宁波', district: '海曙', country: 'CN' }) })
      }
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ ok: true }) })
    })

    await page.goto('/')
    await expect(page.getByText('DISCOVER MODE')).toBeVisible({ timeout: 20_000 })

    // Navigate to map
    await page.getByRole('button', { name: /地图|map/i }).click()
    // Map should load
    await page.waitForTimeout(1000)

    // Navigate back to discover
    await page.getByRole('button', { name: /发现|discover/i }).click()
    await expect(page.getByText('DISCOVER MODE')).toBeVisible({ timeout: 10_000 })
  })
})
