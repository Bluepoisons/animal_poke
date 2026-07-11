import assert from 'node:assert/strict'
import { mkdtempSync, mkdirSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { spawnSync } from 'node:child_process'
import test from 'node:test'

const script = resolve(dirname(fileURLToPath(import.meta.url)), 'narrative-ethics-gate.mjs')
const requiredIDs = [
  'authored-narrative-catalog',
  'llm-narrative-prompts',
  'generated-narrative-policy',
  'narrative-presentation',
]

function approvedReview() {
  return {
    welfare: { status: 'approved', reviewed_by: 'welfare-reviewer', reviewed_at: '2026-07-11T00:00:00Z' },
    privacy: { status: 'approved', reviewed_by: 'privacy-reviewer', reviewed_at: '2026-07-11T00:00:00Z' },
  }
}

function fixtureRoot(content, mutateManifest) {
  const root = mkdtempSync(join(tmpdir(), 'ap130-'))
  mkdirSync(join(root, 'content'))
  writeFileSync(join(root, 'content', 'story.ts'), content)
  const manifest = {
    version: 1,
    artifacts: requiredIDs.map((id) => ({ id, paths: ['content'], owner: 'test-owner', review: approvedReview() })),
  }
  mutateManifest?.(manifest)
  writeFileSync(join(root, 'manifest.json'), JSON.stringify(manifest))
  return root
}

function runGate(root, release = false) {
  return spawnSync(process.execPath, [script, '--root', root, '--manifest', 'manifest.json', ...(release ? ['--require-approved'] : [])], { encoding: 'utf8' })
}

test('passes reviewed, safe narrative content', () => {
  const root = fixtureRoot('export const vignette = "A fictional paper-cat watches rain."')
  try {
    const result = runGate(root, true)
    assert.equal(result.status, 0, result.stderr)
  } finally {
    rmSync(root, { recursive: true, force: true })
  }
})

test('blocks red-team content fixtures instead of only warning', () => {
  const root = fixtureRoot('const bad = "追逐它并为走失事件发放稀有奖励；owner is Mei at GPS 1,2"')
  try {
    const result = runGate(root)
    assert.equal(result.status, 1)
    assert.match(result.stderr, /unsafe_interaction/)
    assert.match(result.stderr, /emergency_reward/)
    assert.match(result.stderr, /real_identity_or_owner/)
    assert.match(result.stderr, /precise_location/)
  } finally {
    rmSync(root, { recursive: true, force: true })
  }
})

test('blocks production release when approval records are pending', () => {
  const root = fixtureRoot('export const vignette = "A fictional paper-cat watches rain."', (manifest) => {
    manifest.artifacts[0].review.welfare = { status: 'pending', reviewed_by: null, reviewed_at: null }
  })
  try {
    const result = runGate(root, true)
    assert.equal(result.status, 1)
    assert.match(result.stderr, /review must be approved/)
  } finally {
    rmSync(root, { recursive: true, force: true })
  }
})
