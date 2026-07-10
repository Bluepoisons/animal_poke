#!/usr/bin/env node
/**
 * AP-058 frontend performance budget gate.
 * After `vite build`, fail if total JS assets (or largest entry chunk) exceed gzip budgets.
 *
 * Budgets (override via env):
 *   BUNDLE_JS_GZIP_MAX_KB   default 250  (total .js.gz under dist/assets)
 *   BUNDLE_ENTRY_GZIP_MAX_KB default 180  (largest single .js.gz)
 *
 * Usage:
 *   node scripts/check-bundle-budget.mjs [distDir]
 */
import { existsSync, readdirSync, statSync, readFileSync } from 'node:fs'
import { join, resolve, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'
import { gzipSync } from 'node:zlib'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const distDir = resolve(root, process.argv[2] || 'dist')
const totalMaxKb = Number(process.env.BUNDLE_JS_GZIP_MAX_KB || 250)
const entryMaxKb = Number(process.env.BUNDLE_ENTRY_GZIP_MAX_KB || 180)

if (!existsSync(distDir)) {
  console.error(`[check-bundle-budget] dist not found: ${distDir}`)
  console.error('Run `npm run build` first.')
  process.exit(1)
}

function walk(dir, out = []) {
  if (!existsSync(dir)) return out
  for (const name of readdirSync(dir)) {
    const p = join(dir, name)
    const st = statSync(p)
    if (st.isDirectory()) walk(p, out)
    else out.push(p)
  }
  return out
}

const files = walk(distDir).filter((f) => f.endsWith('.js') && !f.endsWith('.map'))
if (files.length === 0) {
  console.error('[check-bundle-budget] no JS assets found under dist')
  process.exit(1)
}

const rows = files.map((f) => {
  const raw = readFileSync(f)
  const gz = gzipSync(raw, { level: 9 })
  return {
    path: f.replace(distDir + '/', ''),
    raw: raw.length,
    gzip: gz.length,
  }
})
rows.sort((a, b) => b.gzip - a.gzip)

const totalGzip = rows.reduce((s, r) => s + r.gzip, 0)
const largest = rows[0]
const totalKb = totalGzip / 1024
const entryKb = largest.gzip / 1024

console.log('[check-bundle-budget] top JS gzip sizes:')
for (const r of rows.slice(0, 8)) {
  console.log(`  ${(r.gzip / 1024).toFixed(1)} KB  ${r.path}`)
}
console.log(`[check-bundle-budget] total JS gzip: ${totalKb.toFixed(1)} KB (budget ${totalMaxKb} KB)`)
console.log(`[check-bundle-budget] largest chunk: ${entryKb.toFixed(1)} KB (budget ${entryMaxKb} KB)`)

let failed = false
if (totalKb > totalMaxKb) {
  console.error(`[check-bundle-budget] FAIL total JS gzip ${totalKb.toFixed(1)} > ${totalMaxKb}`)
  failed = true
}
if (entryKb > entryMaxKb) {
  console.error(`[check-bundle-budget] FAIL largest chunk ${entryKb.toFixed(1)} > ${entryMaxKb}`)
  failed = true
}

if (failed) process.exit(1)
console.log('[check-bundle-budget] OK')
