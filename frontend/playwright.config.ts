import { defineConfig, devices } from '@playwright/test'

/**
 * AP-014 / AP-040 / AP-075 E2E — production entry via Vite + page.route API mocks.
 * Matrix: Chromium (CI hard gate) + WebKit (desktop Safari proxy).
 * Mobile projects available via PLAYWRIGHT_MOBILE=1.
 *
 * Uses vite dev server by default (stable under bulk-delete sandboxes).
 * Set E2E_USE_PREVIEW=1 to build + vite preview instead.
 */
const usePreview = process.env.E2E_USE_PREVIEW === '1'
const enableMobile = process.env.PLAYWRIGHT_MOBILE === '1'
const ci = !!process.env.CI

const projects: { name: string; use: (typeof devices)[string] }[] = [
  {
    name: 'chromium',
    use: { ...devices['Desktop Chrome'] },
  },
]

// CI installs Chromium only; enable WebKit explicitly with PLAYWRIGHT_WEBKIT=1.
if (process.env.PLAYWRIGHT_WEBKIT === '1') {
  projects.push({
    name: 'webkit',
    use: { ...devices['Desktop Safari'] },
  })
}

if (enableMobile) {
  projects.push(
    {
      name: 'mobile-chrome',
      use: { ...devices['Pixel 7'] },
    },
    {
      name: 'mobile-safari',
      use: { ...devices['iPhone 14'] },
    },
  )
}

export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  forbidOnly: ci,
  retries: ci ? 1 : 0,
  workers: 1,
  reporter: ci
    ? [['github'], ['list'], ['html', { outputFolder: 'playwright-report', open: 'never' }]]
    : 'list',
  timeout: 90_000,
  expect: { timeout: 15_000 },
  use: {
    baseURL: 'http://127.0.0.1:4173',
    trace: ci ? 'on-first-retry' : 'retain-on-failure',
    video: ci ? 'retain-on-failure' : 'off',
    screenshot: 'only-on-failure',
    locale: 'zh-CN',
    launchOptions: {
      args: [
        '--use-fake-ui-for-media-stream',
        '--use-fake-device-for-media-stream',
      ],
    },
  },
  projects,
  webServer: {
    command: usePreview
      ? 'npx vite build --outDir dist-e2e --emptyOutDir false && npx vite preview --outDir dist-e2e --host 127.0.0.1 --port 4173 --strictPort'
      : 'npx vite --host 127.0.0.1 --port 4173 --strictPort',
    url: 'http://127.0.0.1:4173',
    reuseExistingServer: !ci,
    timeout: 180_000,
    env: {
      ...process.env,
      VITE_API_BASE_URL: '',
      VITE_VISION_MOCK: '0',
    },
  },
  // AP-075: snapshot base directory for visual regression
  snapshotDir: './e2e/__screenshots__',
  snapshotPathTemplate: '{snapshotDir}/{testFilePath}/{arg}{ext}',
})
