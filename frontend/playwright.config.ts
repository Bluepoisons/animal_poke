import { defineConfig, devices } from '@playwright/test'

/**
 * AP-014 / AP-040 E2E — production entry via Vite + page.route API mocks.
 * Matrix: Chromium (CI hard gate) + WebKit (desktop Safari proxy).
 * Mobile projects available via PLAYWRIGHT_MOBILE=1.
 *
 * Uses vite dev server by default (stable under bulk-delete sandboxes).
 * Set E2E_USE_PREVIEW=1 to build + vite preview instead.
 */
const usePreview = process.env.E2E_USE_PREVIEW === '1'
const enableMobile = process.env.PLAYWRIGHT_MOBILE === '1'

const projects = [
  {
    name: 'chromium',
    use: { ...devices['Desktop Chrome'] },
  },
  {
    name: 'webkit',
    use: { ...devices['Desktop Safari'] },
  },
]

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
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  reporter: process.env.CI ? [['github'], ['list']] : 'list',
  timeout: 90_000,
  expect: { timeout: 15_000 },
  use: {
    baseURL: 'http://127.0.0.1:4173',
    trace: 'on-first-retry',
    video: 'off',
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
    reuseExistingServer: !process.env.CI,
    timeout: 180_000,
    env: {
      ...process.env,
      VITE_API_BASE_URL: '',
      VITE_VISION_MOCK: '0',
    },
  },
})
