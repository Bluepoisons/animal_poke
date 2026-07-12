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
    test.setTimeout(120_000)
    const log = createApiCallLog()
    // AP-066: keep tutorial fresh so event-driven coach is exercised
    await installBrowserMocks(page, { completeOnboarding: false })
    await installApiMocks(page, log)

    await page.goto('/')

    // Privacy consent gate (ConsentGate) — rationale comes AFTER grant (AP-066)
    await expect(page.getByRole('heading', { name: /隐私与权限/ })).toBeVisible()
    await page.getByRole('button', { name: /同意并继续/ }).click()

    // Event-driven onboarding: rationale modal before camera/scan
    await expect(page.getByTestId('onboarding-overlay')).toBeVisible({ timeout: 15_000 })
    await expect(page.getByTestId('onboarding-overlay')).toHaveAttribute(
      'data-onboarding-step',
      'rationale',
    )
    await page.getByTestId('onboarding-continue').click()
    await expect(page.getByTestId('onboarding-overlay')).toHaveAttribute(
      'data-onboarding-step',
      'train_scan',
    )

    // Axe scan on discover screen
    const discoverAxe = await scanA11y(page, '[data-testid="discover-screen"]')
    expect(discoverAxe.violations, `a11y violations on discover:\n${discoverAxe.details}`).toBe(0)

    await expect(page.getByText('DISCOVER MODE')).toBeVisible({ timeout: 20_000 })

    // Perform scan — advances train_scan → select_target on detect_success
    const scanBtn = page.getByRole('button', { name: /开始识别/ })
    await expect(scanBtn).toBeVisible({ timeout: 10_000 })
    await expect.poll(async () => scanBtn.isDisabled(), { timeout: 15_000 }).toBe(false)
    await scanBtn.click()

    // Enter capture
    const enterBtn = page.getByTestId('enter-capture').or(page.getByRole('button', { name: /进入捕获/ }))
    await expect(enterBtn).toBeVisible({ timeout: 20_000 })
    await expect(enterBtn).toBeEnabled()
    // DOM click (not force coordinates) — force can hit BottomTabBar over the CTA
    await enterBtn.evaluate((el: HTMLElement) => {
      el.scrollIntoView({ block: 'center', inline: 'center' })
      el.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, view: window }))
    })
    await expect.poll(async () => page.evaluate(() => location.hash), { timeout: 10_000 }).toBe('#capture')
    await expect(page.getByTestId('capture-screen')).toBeVisible({ timeout: 20_000 })
    await expect(page.getByText(/cat/i).first()).toBeVisible()

    // Perform capture
    await page.getByTestId('capture-stage').click()
    await expect(page.getByText(/捕获成功/).first()).toBeVisible({ timeout: 30_000 })

    // AP-066: after training capture → reveal coach; open pokedex via CTA or tab
    const revealCta = page.getByTestId('onboarding-continue')
    if (await revealCta.isVisible().catch(() => false)) {
      await revealCta.click()
    } else {
      await page.getByRole('button', { name: /图鉴/ }).click()
    }
    await expect(page.getByTestId('pokedex-screen')).toBeVisible({ timeout: 15_000 })
    // Tutorial should complete once pokedex opens
    await expect(page.getByTestId('onboarding-overlay')).toHaveCount(0, { timeout: 10_000 })

    // Axe scan on pokedex
    const pokedexAxe = await scanA11y(page)
    expect(pokedexAxe.violations, `a11y violations on pokedex:\n${pokedexAxe.details}`).toBe(0)

    // Reload and verify persistence — tutorial must stay completed (no rationale modal)
    await page.reload()
    await expect(page.getByTestId('onboarding-overlay')).toHaveCount(0, { timeout: 15_000 })
    await expect(
      page.getByRole('button', { name: /图鉴|POKEDEX|Collection|发现|Discover/i }).first(),
    ).toBeVisible({ timeout: 20_000 })

    await page.getByRole('button', { name: /图鉴/ }).click()
    await expect(page.getByTestId('pokedex-screen')).toBeVisible({ timeout: 10_000 })
  })
})
