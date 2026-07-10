#!/usr/bin/env node
/**
 * CI gate: economy Monte Carlo invariants (AP-051).
 * Fails process if any archetype/horizon simulation breaches invariants.
 *
 * Usage: node ./scripts/economy-monte-carlo-gate.mjs
 * Prefer: npm test -- src/economy/monteCarlo (vitest). This script shells vitest.
 */
import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import path from 'node:path'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const r = spawnSync(
  process.platform === 'win32' ? 'npx.cmd' : 'npx',
  ['vitest', 'run', 'src/economy/monteCarlo'],
  { cwd: root, stdio: 'inherit', env: process.env },
)
if (r.status !== 0) {
  console.error('[economy-monte-carlo-gate] FAILED — invariant breach or test error')
  process.exit(r.status ?? 1)
}
console.log('[economy-monte-carlo-gate] OK — no invariant breaches')
process.exit(0)
