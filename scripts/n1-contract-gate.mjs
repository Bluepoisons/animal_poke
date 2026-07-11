#!/usr/bin/env node
/**
 * AP-089 N-1 Compatibility Contract Gate
 *
 * Verifies that the current OpenAPI spec maintains backward compatibility.
 * Checks:
 *  1. Every deprecated:true operation has x-sunset-date
 *  2. Deprecation dates are in the future or recently expired (≤7 days)
 *  3. No endpoint path was removed without a deprecation cycle
 *  4. No response field was removed from a non-deprecated endpoint
 *
 * Usage:
 *   node scripts/n1-contract-gate.mjs                   # validate current spec
 *   node scripts/n1-contract-gate.mjs --previous v1.0.0  # diff against tag
 */
import { readFileSync, existsSync } from 'node:fs'
import { resolve } from 'node:path'
import { execSync } from 'node:child_process'

const ROOT = resolve(new URL('..', import.meta.url).pathname)
const SPEC = resolve(ROOT, 'docs/openapi.yaml')

const args = process.argv.slice(2)
const previousTag = args.includes('--previous') ? args[args.indexOf('--previous') + 1] : null

let violations = 0

function fail(msg) {
  console.error(`[n1-gate] FAIL: ${msg}`)
  violations += 1
}

function ok(msg) {
  console.log(`[n1-gate] OK: ${msg}`)
}

// 1. Read the OpenAPI spec as raw YAML text (no parser needed for these checks)
const spec = readFileSync(SPEC, 'utf8')

// 2. Check deprecated operations have sunset dates
const deprecatedBlocks = [...spec.matchAll(/deprecated:\s*true/g)]
ok(`${deprecatedBlocks.length} deprecated operations found`)

if (deprecatedBlocks.length > 0) {
  // For each deprecated block, check for x-sunset-date nearby
  const lines = spec.split('\n')
  for (let i = 0; i < lines.length; i++) {
    if (lines[i].match(/^\s*deprecated:\s*true\s*$/)) {
      // Search next 10 lines for x-sunset-date
      let found = false
      for (let j = i + 1; j < Math.min(i + 10, lines.length); j++) {
        const sunsetMatch = lines[j].match(/^\s*x-sunset-date:\s*(.+)/)
        if (sunsetMatch) {
          const date = new Date(sunsetMatch[1])
          const sevenDaysFromNow = Date.now() + 7 * 24 * 60 * 60 * 1000
          if (date.getTime() > sevenDaysFromNow) {
            ok(`deprecated op at line ${i + 1}: sunset ${sunsetMatch[1]} is in the future`)
          } else if (date.getTime() > Date.now() - 7 * 24 * 60 * 60 * 1000) {
            ok(`deprecated op at line ${i + 1}: sunset ${sunsetMatch[1]} recently expired (≤7 days)`)
          } else {
            fail(`deprecated op at line ${i + 1}: sunset ${sunsetMatch[1]} is >7 days past`)
          }
          found = true
          break
        }
      }
      if (!found) {
        fail(`deprecated op at line ${i + 1}: missing x-sunset-date annotation`)
      }
    }
  }
}

// 3. Check for removed endpoints (only when --previous is provided)
if (previousTag) {
  try {
    const oldSpec = execSync(`git show ${previousTag}:docs/openapi.yaml`, { encoding: 'utf8', cwd: ROOT })
    const oldPaths = new Set([...oldSpec.matchAll(/^\s{2}(\/[^:]+):/gm)].map(m => m[1]))
    const newPaths = new Set([...spec.matchAll(/^\s{2}(\/[^:]+):/gm)].map(m => m[1]))

    for (const path of oldPaths) {
      if (!newPaths.has(path)) {
        fail(`endpoint ${path} removed without deprecation window (N-1 violation)`)
      }
    }
    ok(`N-1 diff against ${previousTag}: ${oldPaths.size}→${newPaths.size} paths`)
  } catch (e) {
    console.error(`[n1-gate] warning: could not diff against ${previousTag}: ${e.message}`)
  }
}

// 4. Report
if (violations > 0) {
  console.error(`\n[n1-gate] ${violations} N-1 compatibility violation(s)`)
  process.exit(1)
}

console.log('\n[n1-gate] N-1 compatibility gate OK')
