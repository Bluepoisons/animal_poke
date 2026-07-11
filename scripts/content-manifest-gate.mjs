#!/usr/bin/env node
/**
 * AP-110 Content Manifest Publishing Gate
 *
 * Validates authored content before publication:
 *   1. Species pack consistency (frontend ↔ backend mirror)
 *   2. i18n key coverage across locales
 *   3. Cross-reference integrity (species in quests, hunt, animals)
 *   4. Reward/item legality
 *
 * Usage:
 *   node scripts/content-manifest-gate.mjs
 */

import { readFileSync, existsSync } from 'node:fs'
import { resolve, dirname, relative } from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = dirname(fileURLToPath(import.meta.url))
const ROOT = resolve(__dirname, '..')

let violations = 0
let warnings = 0

function fail(msg) { console.error(`[manifest] FAIL: ${msg}`); violations++ }
function warn(msg) { console.warn(`[manifest] WARN: ${msg}`); warnings++ }
function ok(msg) { console.log(`[manifest] OK: ${msg}`) }

// --- 1. Species pack consistency ---
const frontendPacks = resolve(ROOT, 'frontend/src/species/packs.ts')
const backendPacks = resolve(ROOT, 'backend/internal/speciespack/builtin.go')

if (!existsSync(frontendPacks)) fail('frontend species packs file missing')
else if (!existsSync(backendPacks)) fail('backend species packs file missing')
else {
  const fp = readFileSync(frontendPacks, 'utf8')
  const bp = readFileSync(backendPacks, 'utf8')

  // Extract content IDs: frontend uses 'species.cat', backend uses "species.cat"
  const front = [...fp.matchAll(/contentId:\s*['"]([^'"]+)['"]/g)].map(m => m[1])
  const back = [...bp.matchAll(/ContentID:\s*"([^"]+)"/g)].map(m => m[1])
  const frontSet = new Set(front), backSet = new Set(back)

  for (const id of frontSet) if (!backSet.has(id)) fail(`species "${id}" frontend-only`)
  for (const id of backSet) if (!frontSet.has(id)) fail(`species "${id}" backend-only`)
  ok(`species pack sync: ${frontSet.size} front, ${backSet.size} back`)

  // Capturable count: frontend uses literal, backend uses Go constants
  const fCap = (fp.match(/status:\s*['"]capturable['"]/g) || []).length
  const bCap = (bp.match(/Status:\s*StatusCapturable\b/g) || []).length
  if (fCap === bCap) ok(`capturable species: ${bCap}`)
  else fail(`capturable mismatch: front=${fCap} back=${bCap}`)
  ok(`species catalog ready for manifest`)
}

// Known species IDs (both dotted content IDs and short IDs)
const knownContentIds = new Set()
const knownShortIds = new Set()
for (const src of [frontendPacks, backendPacks]) {
  if (existsSync(src)) {
    const c = readFileSync(src, 'utf8')
    for (const m of c.matchAll(/(?:contentId|ContentID):\s*['"]([^'"]+)['"]/g)) knownContentIds.add(m[1])
    for (const m of c.matchAll(/\bID:\s*"([^"]+)"/g)) knownShortIds.add(m[1])
  }
}

// --- 2. i18n key coverage ---
const i18nDir = resolve(ROOT, 'frontend/src/i18n/locales')
// Production locales only; incomplete ja is intentionally hidden (AP-069).
const locales = ['zh', 'en'].filter(l => existsSync(resolve(i18nDir, `${l}.ts`)))

if (locales.length === 0) fail('no locale files in frontend/src/i18n/locales')
else {
  const data = {}
  for (const loc of locales) {
    const content = readFileSync(resolve(i18nDir, `${loc}.ts`), 'utf8')
    data[loc] = new Set([...content.matchAll(/^\s*'([\w.]+)':\s*[/`'"]/gm)].map(m => m[1]))
  }
  const ref = data['zh'] || data[locales[0]]
  if (ref) {
    for (const loc of locales) {
      const missing = [...ref].filter(k => !data[loc]?.has(k))
      const extra = [...(data[loc] || [])].filter(k => !ref.has(k))
      if (missing.length > 0) fail(`${loc} missing ${missing.length} keys: ${missing.slice(0,5).join(', ')}${missing.length>5?'...':''}`)
      if (extra.length > 0) warn(`${loc} has ${extra.length} extra keys: ${extra.slice(0,5).join(', ')}${extra.length>5?'...':''}`)
    }
    ok(`i18n: ${ref.size} ref keys across ${locales.length} locales`)
  }
}

// --- 3. Cross-reference integrity ---
const questCatalog = resolve(ROOT, 'backend/internal/questcatalog/catalog.go')
if (existsSync(questCatalog)) {
  const qc = readFileSync(questCatalog, 'utf8')
  const refs = [...qc.matchAll(/Species:\s*"([^"]+)"/g)].map(m => m[1]).filter(Boolean)
  for (const s of refs) {
    if (!knownShortIds.has(s) && !knownContentIds.has(s)) fail(`quest species "${s}" undefined`)
  }
  ok(`quest species refs: ${refs.length} valid`)

  // Reward legality
  const shopPath = resolve(ROOT, 'frontend/src/shop/constants.ts')
  const items = new Set()
  if (existsSync(shopPath)) {
    for (const m of readFileSync(shopPath, 'utf8').matchAll(/id:\s*['"]([^'"]+)['"]/g)) items.add(m[1])
  }
  for (const r of [...qc.matchAll(/Reward:\s*"([^"]+)"/g)].map(m => m[1])) {
    if (!items.has(r) && !['gold','exp','stamina'].includes(r)) warn(`quest reward "${r}" not a known item`)
  }

  // Effect namespace check
  const ns = new Set([...qc.matchAll(/"([a-z_]+):/g)].map(m => m[1]))
  const allowed = new Set(['flag','rel','clue','knowledge','reward','item','gold','exp','stamina','skill','stat','species','count','type','min_level','max_level','quest_id','stage','city','weather','hour','day','hour_range','not_before','not_after','certified','id','key','value','from','to'])
  for (const n of ns) if (!allowed.has(n) && n.length < 25) warn(`effect namespace "${n}" not in allowed set`)
  ok(`quest effect namespaces scanned`)
}

// --- 4. Legacy animal data refs ---
for (const p of ['frontend/src/features/animal-poke/data/animals.ts', 'frontend/src/types.ts']) {
  const full = resolve(ROOT, p)
  if (!existsSync(full)) continue
  const content = readFileSync(full, 'utf8')
  for (const m of content.matchAll(/species:\s*['"]([^'"]+)['"]/g)) {
    const s = m[1]
    if (s && !knownShortIds.has(s) && !knownContentIds.has(s)) {
      warn(`${p}: species "${s}" not in pack registry`)
    }
  }
}

// --- 5. Summary ---
console.log()
if (violations > 0) {
  console.error(`[manifest] ${violations} violation(s), ${warnings} warning(s)`)
  process.exit(1)
}
console.log(`[manifest] content manifest gate OK (${warnings} warning(s))`)
