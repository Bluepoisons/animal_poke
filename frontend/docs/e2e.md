# E2E hard gate (AP-014 / #175)

Production entry under test: `App` → `features/animal-poke/AnimalPokeApp`.

## What is covered

Full capture loop with **Playwright + `page.route` API mocks** (CI-friendly, no MySQL required):

1. Consent gate → device auth
2. Camera mock → detect (`POST /api/v1/vision/detect`)
3. Enter capture → settle (stamina once)
4. Analyze → value → IndexedDB → sync (`/api/v1/vision/analyze`, `/api/v1/value/generate`, `/api/v1/sync/animal`)
5. Refresh restore: collection persists, sync queue empty
6. Broken core detect API must fail the gate path (no capture)

Optional browser mocks: camera, geolocation, permission denial, forced capture success via `window.__AP_FORCE_CAPTURE_SUCCESS`.

## Prerequisites

```bash
cd frontend
npm ci
npx playwright install chromium webkit
```

Node 22 LTS required (see `package.json` engines).

## Run locally

```bash
# full local e2e (Vite dev server + Chromium)
npm run test:e2e

# CI-equivalent browser matrix
CI=1 PLAYWRIGHT_WEBKIT=1 npm run test:e2e

# production artifact entry
E2E_USE_PREVIEW=1 PLAYWRIGHT_WEBKIT=1 npm run test:e2e

# headed
npm run test:e2e -- --headed

# single file
npx playwright test e2e/capture-loop.spec.ts
```

Config: `playwright.config.ts`  
Mocks: `e2e/helpers/mocks.ts`  
Specs: `e2e/*.spec.ts`

## CI

GitHub Actions job **Frontend E2E** runs Chromium + WebKit and **fails the PR** if the core flow breaks. Frontend unit tests no longer use `continue-on-error`.

## Full docker stack (optional, local)

For a non-mocked backend with `AI_MOCK_ENABLED=true`:

```bash
# from repo root
cd backend && docker compose up -d   # MySQL + backend if available
cd ../frontend
VITE_API_BASE_URL=http://127.0.0.1:8080 npm run dev
# then run playwright against dev server (override baseURL)
```

CI path intentionally uses **route mocks** so the hard gate stays fast and deterministic.
