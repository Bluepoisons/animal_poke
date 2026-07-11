#!/usr/bin/env node
/**
 * AP-075: one-command bootstrap for Playwright E2E.
 *
 *   node scripts/e2e-bootstrap.mjs           # install + run
 *   node scripts/e2e-bootstrap.mjs --webkit   # also install WebKit
 *   node scripts/e2e-bootstrap.mjs --only-install
 *   node scripts/e2e-bootstrap.mjs --only-test
 */
import { execSync } from 'node:child_process'
import { existsSync } from 'node:fs'
import { resolve } from 'node:path'

const root = new URL('..', import.meta.url).pathname

const args = new Set(process.argv.slice(2))
const webkit = args.has('--webkit') || args.has('--all')
const onlyInstall = args.has('--only-install')
const onlyTest = args.has('--only-test')
const all = onlyInstall && onlyTest ? false : true // default: both

function run(cmd, opts = {}) {
  console.log(`\n→ ${cmd}`)
  execSync(cmd, { cwd: root, stdio: 'inherit', ...opts })
}

// 1. Install dependencies (always)
if (!existsSync(resolve(root, 'node_modules/.package-lock.json'))) {
  run('npm ci')
}

// 2. Install Playwright browsers
if (all || onlyInstall) {
  const browsers = webkit ? 'chromium webkit' : 'chromium'
  run(`npx playwright install --with-deps ${browsers}`)
  console.log('\n✓ E2E browsers installed.')
}

// 3. Run tests
if (all || onlyTest) {
  if (webkit) {
    run('PLAYWRIGHT_WEBKIT=1 npx playwright test', { env: { ...process.env, PLAYWRIGHT_WEBKIT: '1' } })
  } else {
    run('npx playwright test')
  }
  console.log('\n✓ E2E tests passed.')
}
