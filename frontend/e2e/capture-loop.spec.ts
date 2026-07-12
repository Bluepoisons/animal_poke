import { test, expect } from '@playwright/test'
import {
  createApiCallLog,
  installApiMocks,
  installBrowserMocks,
} from './helpers/mocks'

/**
 * AP-014 production entry hard gate
 * Flow: consent → auth → detect → capture → analyze → value → IDB → sync
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
            pending = items.filter((i) => i.status === 'pending' || i.status === 'failed').length
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
    const reset = page.getByRole('button', { name: /重新开始/ })
    if (await reset.isVisible().catch(() => false)) {
      await reset.click()
    }
    await page.waitForTimeout(500)
  }
  const scanBtn = page.getByRole('button', { name: /开始识别/ })
  await expect(scanBtn).toBeVisible({ timeout: 10_000 })
  await expect
    .poll(async () => scanBtn.isDisabled(), { timeout: 15_000 })
    .toBe(false)
  await scanBtn.click()
}

test.describe('AP-014 production capture hard gate', () => {
  test('full capture loop is single-shot and survives refresh', async ({ page }) => {
    const log = createApiCallLog()
    await installBrowserMocks(page)
    await installApiMocks(page, log)

    await page.goto('/')

    await expect(page.getByRole('heading', { name: /隐私与权限/ })).toBeVisible()
    await page.getByRole('button', { name: /同意并继续/ }).click()

    await expect(page.getByText('DISCOVER MODE')).toBeVisible({ timeout: 20_000 })

    const detectResp = page.waitForResponse(
      (r) => r.url().includes('/api/v1/vision/detect') && r.request().method() === 'POST',
      { timeout: 20_000 },
    )
    await waitCameraReadyAndScan(page)
    await detectResp

    const enterBtn = page.getByRole('button', { name: /进入捕获/ })
    await expect(enterBtn).toBeVisible({ timeout: 20_000 })
    await expect(enterBtn).toBeEnabled()
    // force avoids rare overlay measure thrash blocking actionability
    await enterBtn.click({ force: true })

    await expect(page.getByTestId('capture-screen')).toBeVisible({ timeout: 20_000 })
    await expect(page.getByText(/cat/i).first()).toBeVisible()

    const analyzeResp = page.waitForResponse(
      (r) => r.url().includes('/api/v1/vision/analyze') && r.request().method() === 'POST',
      { timeout: 30_000 },
    )
    const valueResp = page.waitForResponse(
      (r) => r.url().includes('/api/v1/value/generate') && r.request().method() === 'POST',
      { timeout: 30_000 },
    )
    const syncResp = page.waitForResponse(
      (r) => r.url().includes('/api/v1/sync/animal') && r.request().method() === 'POST',
      { timeout: 30_000 },
    )

    await page.getByTestId('capture-stage').click()
    await expect(page.getByText(/捕获成功/).first()).toBeVisible({ timeout: 30_000 })

    const [a, v, s] = await Promise.all([analyzeResp, valueResp, syncResp])
    expect(a.ok()).toBeTruthy()
    expect(v.ok()).toBeTruthy()
    expect(s.ok()).toBeTruthy()

    // Hard gate counters from route mock
    expect(log.auth).toBeGreaterThanOrEqual(1)
    expect(log.consent).toBeGreaterThanOrEqual(1)
    expect(log.detect).toBeGreaterThanOrEqual(1)
    expect(log.analyze).toBeGreaterThanOrEqual(1)
    expect(log.value).toBeGreaterThanOrEqual(1)
    expect(log.sync).toBeGreaterThanOrEqual(1)

    const detectCount = log.detect
    const analyzeCount = log.analyze
    const valueCount = log.value
    const syncCount = log.sync

    // Second capture click must not re-call core generation APIs
    await page.getByTestId('capture-stage').click()
    await page.waitForTimeout(500)
    expect(log.detect).toBe(detectCount)
    expect(log.analyze).toBe(analyzeCount)
    expect(log.value).toBe(valueCount)
    expect(log.sync).toBe(syncCount)

    await page.getByRole('button', { name: /图鉴/ }).click()

    const beforeReload = await readIdbSnapshot(page)
    expect(beforeReload.animals).toBeGreaterThanOrEqual(1)
    expect(beforeReload.pending).toBe(0)

    await page.reload()
    // After i18n (AP-069) default locale is zh; multiple nodes may contain 图鉴
    // (route announcer + tab + subtitle). Use .first() to avoid strict-mode multi-match.
    await expect(
      page.getByRole('button', { name: /图鉴|POKEDEX|Collection/i }).or(page.getByText('DISCOVER MODE')).first(),
    ).toBeVisible({
      timeout: 20_000,
    })

    const afterReload = await readIdbSnapshot(page)
    expect(afterReload.animals).toBeGreaterThanOrEqual(1)
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
    await expect(page.getByText('DISCOVER MODE')).toBeVisible({ timeout: 20_000 })

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
    // AP-064/065: denied reason may appear in status pill or settings help
    await expect(
      page.getByTestId('camera-status-pill').or(page.getByTestId('camera-settings-help')).or(page.getByText(/权限|denied|设置/i)),
    ).toBeVisible({ timeout: 20_000 })
    await expect(page.getByTestId('camera-retry').or(page.getByTestId('camera-settings-help')).or(page.getByTestId('camera-placeholder'))).toBeVisible()
  })
})
