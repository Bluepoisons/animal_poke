import { test, expect } from '@playwright/test'
import { installBrowserMocks } from './helpers/mocks'

async function prepareBattleRoute(page: import('@playwright/test').Page, unlocked: boolean) {
  await installBrowserMocks(page)
  await page.addInitScript((battleUnlocked) => {
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
    if (battleUnlocked) {
      localStorage.setItem(
        'animal_poke_stamina',
        JSON.stringify({
          level: 2,
          exp: 100,
          currentStamina: 120,
          totalCaptures: 1,
          lastRecoverTime: Date.now(),
          gold: 0,
          potionPurchasesToday: 0,
          potionPurchaseDate: new Date().toISOString().slice(0, 10),
          totalBattlesWon: 0,
          totalBattles: 0,
          currentWinStreak: 0,
          maxWinStreak: 0,
        }),
      )
    }
  }, unlocked)
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
}

test.describe('battle smoke loop', () => {
  test('locked deep link returns a new player to discover', async ({ page }) => {
    await prepareBattleRoute(page, false)

    await page.goto('/#battle')

    await expect(page).toHaveURL(/#discover$/)
    await expect(page.getByText(/发现模式|DISCOVER MODE/)).toBeVisible({ timeout: 15_000 })
  })

  test('auto battle advances beyond the initial round', async ({ page }) => {
    await prepareBattleRoute(page, true)

    await page.goto('/#battle')
    await expect(page.getByText('BATTLE')).toBeVisible({ timeout: 15_000 })
    const phone = page.locator('[data-phone-frame]')
    await expect(phone).toHaveCSS('border-top-width', '3px')
    await expect
      .poll(() => page.locator('.ap-page-right').textContent(), { timeout: 10_000 })
      .toMatch(/第 [2-9] 回合|结果/)

    const stamina = await page.evaluate(() => {
      const raw = localStorage.getItem('animal_poke_stamina')
      return raw ? JSON.parse(raw).currentStamina : null
    })
    expect(stamina).toBe(100)
  })
})
