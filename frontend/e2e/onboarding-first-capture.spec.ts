import { test, expect } from '@playwright/test'
import {
  createApiCallLog,
  installApiMocks,
  installBrowserMocks,
} from './helpers/mocks'
import { scanA11y } from './helpers/axe'

/**
 * AP-075 onboarding → first-capture → pokedex gate
 * Covers: consent grant, first scan+capture, pokedex entry visible, reload persistence.
 */

test.describe('AP-075 onboarding to first capture', () => {
  test('first-launch consent → capture → pokedex → survives reload', async ({ page }) => {
    const log = createApiCallLog()
    await installBrowserMocks(page)
    await installApiMocks(page, log)

    await page.goto('/')

    // Privacy consent gate
    await expect(page.getByRole('heading', { name: /隐私与权限/ })).toBeVisible()
    await page.getByRole('button', { name: /同意并继续/ }).click()

    // Axe scan on discover screen
    const discoverAxe = await scanA11y(page, '[data-testid="discover-screen"]')
    expect(discoverAxe.violations, `a11y violations on discover:\n${discoverAxe.details}`).toBe(0)

    await expect(page.getByText('DISCOVER MODE')).toBeVisible({ timeout: 20_000 })

    // Perform scan
    const scanBtn = page.getByRole('button', { name: /开始识别/ })
    await expect(scanBtn).toBeVisible({ timeout: 10_000 })
    await expect.poll(async () => scanBtn.isDisabled(), { timeout: 15_000 }).toBe(false)
    await scanBtn.click()

    // Enter capture
    const enterBtn = page.getByTestId('enter-capture').or(page.getByRole('button', { name: /进入捕获/ }))
    await expect(enterBtn).toBeVisible({ timeout: 20_000 })
    await expect(enterBtn).toBeEnabled()
    await enterBtn.click({ force: true })

    await expect(page.getByTestId('capture-screen')).toBeVisible()
    await expect(page.getByText(/cat/i).first()).toBeVisible()

    // Perform capture
    await page.getByTestId('capture-stage').click()
    await expect(page.getByText(/捕获成功/).first()).toBeVisible({ timeout: 30_000 })

    // Navigate to pokedex
    await page.getByRole('button', { name: /图鉴/ }).click()
    await expect(page.getByTestId('pokedex-screen')).toBeVisible({ timeout: 10_000 })

    // Axe scan on pokedex
    const pokedexAxe = await scanA11y(page)
    expect(pokedexAxe.violations, `a11y violations on pokedex:\n${pokedexAxe.details}`).toBe(0)

    // Reload and verify persistence (zh default may land on 图鉴)
    await page.reload()
    await expect(
      page.getByRole('button', { name: /图鉴|POKEDEX|Collection|发现|Discover/i }).first(),
    ).toBeVisible({ timeout: 20_000 })

    await page.getByRole('button', { name: /图鉴/ }).click()
    await expect(page.getByTestId('pokedex-screen')).toBeVisible({ timeout: 10_000 })
  })
})
