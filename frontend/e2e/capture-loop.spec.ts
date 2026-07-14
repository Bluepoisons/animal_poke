import { test, expect } from '@playwright/test'
import {
  createApiCallLog,
  installApiMocks,
  installBrowserMocks,
} from './helpers/mocks'

/**
 * AP-014 production entry hard gate
 * Flow: consent → auth → detect → capture → analyze → value → IDB
 * After reload: collection persists; queue empty; no double reward.
 */

const DB_NAME = 'animal-poke-db'

async function readIdbSnapshot(page: import('@playwright/test').Page) {
  return page.evaluate(async (dbName) => {
    return new Promise<{ animals: number; pending: number }>((resolve, reject) => {
      const open = indexedDB.open(dbName)
      open.onerror = () => reject(open.error || new Error('idb open failed'))
      open.onsuccess = () => {
        const db = open.result
        const names = Array.from(db.objectStoreNames)
        if (!names.includes('animals')) {
          resolve({ animals: 0, pending: 0 })
          return
        }
        const stores = ['animals', 'sync_queue'].filter((n) => names.includes(n))
        const tx = db.transaction(stores, 'readonly')
        let animals = 0
        let pending = 0
        let left = stores.length
        const done = () => {
          left -= 1
          if (left <= 0) {
            db.close()
            resolve({ animals, pending })
          }
        }
        const a = tx.objectStore('animals').count()
        a.onsuccess = () => {
          animals = a.result
          done()
        }
        a.onerror = () => reject(a.error)
        if (names.includes('sync_queue')) {
          const q = tx.objectStore('sync_queue').getAll()
          q.onsuccess = () => {
            const items = (q.result || []) as Array<{ status?: string }>
            pending = items.filter((i) => i.status !== 'synced').length
            done()
          }
          q.onerror = () => reject(q.error)
        }
      }
    })
  }, DB_NAME)
}

async function waitCameraReadyAndScan(page: import('@playwright/test').Page) {
  const ready = page.getByText(/相机就绪/)
  try {
    await expect(ready).toBeVisible({ timeout: 10_000 })
  } catch {
    const reset = page.getByTestId('camera-retry')
    if (await reset.isVisible().catch(() => false)) {
      await reset.click()
    }
    await page.waitForTimeout(500)
  }
  const scanBtn = page.getByTestId('start-detect')
  await expect(scanBtn).toBeVisible({ timeout: 10_000 })
  await expect
    .poll(async () => scanBtn.isDisabled(), { timeout: 15_000 })
    .toBe(false)
  await scanBtn.click()
}

async function expectCapturedCatCard(page: import('@playwright/test').Page) {
  await expect(page.getByTestId('pokedex-screen')).toBeVisible({ timeout: 15_000 })
  await expect(page.getByRole('button', { name: /Sunny.*少见|Sunny.*uncommon/i })).toBeVisible({ timeout: 15_000 })
  await expect(page.getByText(/游侠.*风属性/).first()).toBeVisible()
  await expect(page.getByText('???', { exact: true })).toHaveCount(0)
}

test.describe('AP-014 production capture hard gate', () => {
  test('full capture loop is single-shot and survives refresh', async ({ page }) => {
    const log = createApiCallLog()
    await installBrowserMocks(page)
    await installApiMocks(page, log)

    await page.goto('/')

    await expect(page.getByRole('heading', { name: /隐私与权限/ })).toBeVisible()
    await page.getByRole('button', { name: /同意并继续/ }).click()

    await expect(page.getByText(/发现模式|DISCOVER MODE/)).toBeVisible({ timeout: 20_000 })

    const detectResp = page.waitForResponse(
      (r) => r.url().includes('/api/v1/vision/detect') && r.request().method() === 'POST',
      { timeout: 20_000 },
    )
    await waitCameraReadyAndScan(page)
    await detectResp

    const enterBtn = page.getByTestId('enter-capture').or(page.getByRole('button', { name: /进入捕获/ }))
    await expect(enterBtn).toBeVisible({ timeout: 20_000 })
    await expect(enterBtn).toBeEnabled()
    // DOM click (not force coordinates) — force can hit BottomTabBar over the CTA
    await enterBtn.evaluate((el: HTMLElement) => {
      el.scrollIntoView({ block: 'center', inline: 'center' })
      el.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, view: window }))
    })
    // Prefer hash change as success signal; capture-screen mounts next
    await expect
      .poll(
        async () =>
          page.evaluate(() => ({
            hash: location.hash,
            hasCapture: !!document.querySelector('[data-testid="capture-screen"]'),
            toast: document.querySelector('.ap-toast')?.textContent || '',
            phasePill: document.querySelector('[data-testid="camera-status-pill"]')?.textContent || '',
          })),
        { timeout: 15_000 },
      )
      .toMatchObject({ hash: '#capture' })
    await expect(page.getByTestId('capture-screen')).toBeVisible({ timeout: 20_000 })
    await expect(page.getByText(/猫|cat/i).first()).toBeVisible()

    const analyzeResp = page.waitForResponse(
      (r) => r.url().includes('/api/v1/vision/analyze') && r.request().method() === 'POST',
      { timeout: 30_000 },
    )
    const valueResp = page.waitForResponse(
      (r) => r.url().includes('/api/v1/value/generate') && r.request().method() === 'POST',
      { timeout: 30_000 },
    )
    const chargeButton = page.getByTestId('charge-button')
    await chargeButton.dispatchEvent('pointerdown', { pointerId: 1 })
    await chargeButton.dispatchEvent('pointerup', { pointerId: 1 })
    await expect(page.getByText(/捕获成功/).first()).toBeVisible({ timeout: 30_000 })

    const [a, v] = await Promise.all([analyzeResp, valueResp])
    expect(a.ok()).toBeTruthy()
    expect(v.ok()).toBeTruthy()

    // Hard gate counters from route mock
    expect(log.auth).toBeGreaterThanOrEqual(1)
    expect(log.consent).toBeGreaterThanOrEqual(1)
    expect(log.detect).toBe(1)
    expect(log.analyze).toBe(1)
    expect(log.value).toBe(1)
    expect(log.sync).toBe(0)

    const detectCount = log.detect
    const analyzeCount = log.analyze
    const valueCount = log.value
    const syncCount = log.sync

    // Second capture click must not re-call core generation APIs
    await chargeButton.dispatchEvent('pointerdown', { pointerId: 2 })
    await chargeButton.dispatchEvent('pointerup', { pointerId: 2 })
    await page.waitForTimeout(500)
    expect(log.detect).toBe(detectCount)
    expect(log.analyze).toBe(analyzeCount)
    expect(log.value).toBe(valueCount)
    expect(log.sync).toBe(syncCount)

    await page.getByRole('button', { name: /图鉴/ }).click()
    await expectCapturedCatCard(page)

    const beforeReload = await readIdbSnapshot(page)
    expect(beforeReload.animals).toBe(1)
    expect(beforeReload.pending).toBe(0)

    await page.reload()
    await expectCapturedCatCard(page)

    const afterReload = await readIdbSnapshot(page)
    expect(afterReload.animals).toBe(1)
    expect(afterReload.pending).toBe(0)
  })

  test('breaking vision detect fails the hard gate path', async ({ page }) => {
    const log = createApiCallLog()
    await installBrowserMocks(page)
    await page.route(/\/api\/v1\//, async (route) => {
      const path = new URL(route.request().url()).pathname
      if (path.includes('/auth/device')) {
        log.auth += 1
        return route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            token: 'e2e-test-token',
            expires_at: new Date(Date.now() + 3600_000).toISOString(),
          }),
        })
      }
      if (path.includes('/privacy/consent')) {
        log.consent += 1
        return route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ ok: true }),
        })
      }
      if (path.includes('/vision/detect')) {
        log.detect += 1
        return route.fulfill({
          status: 500,
          contentType: 'application/json',
          body: JSON.stringify({ error: 'broken_core_api' }),
        })
      }
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ ok: true }),
      })
    })

    await page.goto('/')
    await page.getByRole('button', { name: /同意并继续/ }).click()
    await expect(page.getByText(/发现模式|DISCOVER MODE/)).toBeVisible({ timeout: 20_000 })

    const detectResp = page.waitForResponse(
      (r) => r.url().includes('/api/v1/vision/detect'),
      { timeout: 20_000 },
    )
    await waitCameraReadyAndScan(page)
    await detectResp

    // AP-065: status pill + quality tip may both mention 识别失败 — scope to tip/pill.
    await expect(page.getByTestId('quality-tip')).toBeVisible({ timeout: 15_000 })
    await expect(page.getByTestId('quality-tip')).toContainText(/识别失败|鉴权失败|无法识别|画面中未发现|检测失败/i)
    expect(log.detect).toBeGreaterThanOrEqual(1)
    await expect(page.getByTestId('capture-screen')).toHaveCount(0)
  })

  test('camera permission denial is recoverable messaging', async ({ page }) => {
    const log = createApiCallLog()
    await installApiMocks(page, log)

    await page.addInitScript(() => {
      try {
        localStorage.setItem(
          'animal-poke-onboarding-v1',
          JSON.stringify({ step: 'done', skipped: true, completedAt: Date.now() }),
        )
      } catch {}
      // Do NOT set __AP_FORCE_CAMERA_READY — we want denied path
      Object.defineProperty(navigator, 'mediaDevices', {
        configurable: true,
        value: {
          getUserMedia: async () => {
            const err = new Error('Permission denied')
            err.name = 'NotAllowedError'
            throw err
          },
        },
      })
    })

    await page.goto('/')
    await page.getByRole('button', { name: /同意并继续/ }).click()
    // AP-064/065: denied recovery chrome (avoid multi-match strict mode)
    await expect(page.getByTestId('camera-status-pill')).toBeVisible({ timeout: 20_000 })
    await expect(page.getByTestId('camera-placeholder')).toBeVisible()
    await expect(page.getByTestId('camera-settings-help')).toBeVisible()
  })
})
