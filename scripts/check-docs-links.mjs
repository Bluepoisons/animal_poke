#!/usr/bin/env node
/**
 * AP-060: fail on broken relative markdown links under docs/.
 * Only checks local relative targets (no http/https).
 */
import { readdirSync, readFileSync, statSync, existsSync } from 'node:fs'
import { dirname, join, resolve, normalize } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const docsRoot = join(root, 'docs')

function walk(dir, out = []) {
  for (const name of readdirSync(dir)) {
    if (name === 'node_modules' || name === 'assets') continue
    const p = join(dir, name)
    const st = statSync(p)
    if (st.isDirectory()) walk(p, out)
    else if (name.endsWith('.md')) out.push(p)
  }
  return out
}

const linkRe = /\]\(([^)]+)\)/g
const files = walk(docsRoot)
const broken = []

for (const file of files) {
  const text = readFileSync(file, 'utf8')
  let m
  while ((m = linkRe.exec(text))) {
    let target = m[1].trim()
    if (!target || target.startsWith('http://') || target.startsWith('https://') || target.startsWith('mailto:')) continue
    if (target.startsWith('#')) continue
    // strip anchor
    target = target.split('#')[0]
    if (!target) continue
    if (target.startsWith('/')) continue
    const abs = normalize(resolve(dirname(file), target))
    if (!existsSync(abs)) {
      broken.push({ file: file.replace(root + '/', ''), target })
    }
  }
}

if (broken.length) {
  console.error(`AP-060 docs link check failed (${broken.length}):`)
  for (const b of broken.slice(0, 50)) {
    console.error(`  ${b.file} -> ${b.target}`)
  }
  process.exit(1)
}

console.log(`AP-060 docs link check OK (${files.length} markdown files)`)
