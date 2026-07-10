#!/usr/bin/env node
/**
 * AP-032 dead-module gate.
 * Fails if retired duplicate UI / pipeline / queue paths reappear under frontend/src.
 */
import { existsSync } from 'node:fs'
import { resolve, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')

const banned = [
  'src/components/CaptureScreen.tsx',
  'src/components/CollectScreen.tsx',
  'src/components/DiscoverScreen.tsx',
  'src/components/StoreScreen.tsx',
  'src/components/BattleScreen.tsx',
  'src/components/MapScreen.tsx',
  'src/components/DispatchScreen.tsx',
  'src/components/AchievementScreen.tsx',
  'src/components/TabBar.tsx',
  'src/components/TopBar.tsx',
  'src/sync/queue.ts',
  'src/capture/pipeline.ts',
]

const hits = banned.filter((p) => existsSync(resolve(root, p)))
if (hits.length) {
  console.error('AP-032 dead-module gate failed. Reintroduced banned modules:')
  for (const h of hits) console.error(`  - ${h}`)
  process.exit(1)
}

console.log('AP-032 dead-module gate OK (no banned modules)')
