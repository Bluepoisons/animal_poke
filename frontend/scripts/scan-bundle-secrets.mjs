#!/usr/bin/env node
/**
 * 构建后扫描 dist/ 是否泄漏第三方 Key / Secret 字面量。
 * 允许源码中出现 denylist 关键词（用于防护），只拦截疑似真实密钥值。
 * 用法：node scripts/scan-bundle-secrets.mjs [distDir]
 */
import { readdirSync, readFileSync, statSync, existsSync } from 'node:fs'
import { join, extname } from 'node:path'

const distDir = process.argv[2] || join(process.cwd(), 'dist')
if (!existsSync(distDir)) {
  console.error(`[scan-bundle-secrets] dist not found: ${distDir}`)
  process.exit(1)
}

// 疑似真实密钥字面量
const SUSPICIOUS_VALUE_RES = [
  /sk-[A-Za-z0-9]{20,}/,
  /AKID[A-Za-z0-9]{10,}/,
  /eyJ[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}/, // JWT
  // 赋值形态：VITE_*SECRET* = "longvalue"
  /VITE_[A-Z0-9_]*(SECRET|TOKEN|PASSWORD|API_KEY|MAP_KEY)[A-Z0-9_]*\s*[:=]\s*['"`][^'"`\s]{12,}['"`]/i,
]


// AP-063: production bundle must not ship mock oauth UI strings / prefilled dev credentials.
const PROD_FORBIDDEN_LITERALS = [
  'mock_oauth',
  'Mock OAuth',
  'dev-user',
  'dev-secret-token',
]

const TEXT_EXTS = new Set(['.js', '.css', '.html', '.map', '.json', '.txt', '.svg'])

function walk(dir, out = []) {
  for (const name of readdirSync(dir)) {
    const p = join(dir, name)
    const st = statSync(p)
    if (st.isDirectory()) walk(p, out)
    else out.push(p)
  }
  return out
}

const files = walk(distDir).filter((f) => TEXT_EXTS.has(extname(f)) || extname(f) === '')
const hits = []

for (const file of files) {
  let text
  try {
    text = readFileSync(file, 'utf8')
  } catch {
    continue
  }
  for (const re of SUSPICIOUS_VALUE_RES) {
    if (re.test(text)) hits.push({ file, kind: 'value', re: String(re) })
  }
  // AP-063: forbid mock/dev auth literals in production runtime assets (skip source maps)
  const ext = extname(file)
  if (ext !== '.map') {
    for (const lit of PROD_FORBIDDEN_LITERALS) {
      if (text.includes(lit)) hits.push({ file, kind: 'prod-auth-literal', re: lit })
    }
  }
}

if (hits.length) {
  console.error('[scan-bundle-secrets] FAILED — possible secret leakage in bundle:')
  for (const h of hits) {
    console.error(`  - ${h.kind} ${h.re} in ${h.file}`)
  }
  process.exit(1)
}

console.log(`[scan-bundle-secrets] OK — scanned ${files.length} files under ${distDir}`)
