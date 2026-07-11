#!/usr/bin/env node
/**
 * AP-130: block unsafe narrative content and releases without recorded review.
 *
 * Normal mode verifies ownership metadata and lints published narrative source.
 * --require-approved additionally requires welfare and privacy sign-off; the
 * release workflow uses that mode so a pending review cannot reach production.
 */
import { existsSync, readFileSync, readdirSync, statSync } from 'node:fs'
import { dirname, extname, isAbsolute, relative, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const repositoryRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const args = process.argv.slice(2)
const requireApproved = args.includes('--require-approved')
const rootFlag = args.indexOf('--root')
const manifestFlag = args.indexOf('--manifest')
const root = resolve(rootFlag >= 0 ? args[rootFlag + 1] : repositoryRoot)
const manifestPath = resolve(root, manifestFlag >= 0 ? args[manifestFlag + 1] : 'docs/narrative-ethics-manifest.json')
const requiredArtifactIDs = new Set([
  'authored-narrative-catalog',
  'llm-narrative-prompts',
  'generated-narrative-policy',
  'narrative-presentation',
])
const contentExtensions = new Set(['.go', '.ts', '.tsx', '.json'])
const forbiddenRules = [
  ['real_identity_or_owner', /\b(?:owned by|owner is|lives at|diagnosed|medical record)\b|主人是|病历|确诊|真实.{0,12}(?:经历|情绪|健康|主人|意图)/iu],
  ['precise_location', /\b(?:gps|latitude|longitude|coordinates?|address)\b|(?:经纬度|精确(?:地址|位置|坐标)|家住|住在)/iu],
  ['unsafe_interaction', /\b(?:chase|feed|touch|trespass|night adventure)\b|(?:追逐|投喂|触摸|私闯|夜间冒险)/iu],
  ['emergency_reward', /(?:lost|injured|stray|emergency).{0,48}(?:rare|reward|bonus)|(?:走失|受伤|流浪|紧急).{0,48}(?:稀有|奖励|加成)/iu],
  ['paid_moral_choice', /(?:pay|purchase|premium).{0,32}(?:choice|moral)|(?:付费|购买|高级).{0,32}(?:选择|道德)/iu],
  ['minor_inequity', /(?:minor|child|accessibility|home).{0,48}(?:paywall|premium|locked)|(?:未成年人|儿童|无障碍|居家).{0,48}(?:付费墙|高级|锁定)/iu],
]

function usageError(message) {
  console.error(`AP-130 narrative ethics gate: ${message}`)
  process.exitCode = 1
}

function underRoot(path) {
  const pathRelative = relative(root, path)
  return pathRelative !== '' && !pathRelative.startsWith('..') && !isAbsolute(pathRelative)
}

function filesUnder(path, out = []) {
  const info = statSync(path)
  if (info.isFile()) {
    if (contentExtensions.has(extname(path))) out.push(path)
    return out
  }
  for (const entry of readdirSync(path, { withFileTypes: true })) {
    if (entry.name === 'node_modules' || entry.name === 'dist' || entry.name.startsWith('.')) continue
    filesUnder(resolve(path, entry.name), out)
  }
  return out
}

function reviewIsApproved(review) {
  return review?.status === 'approved' && typeof review.reviewed_by === 'string' && review.reviewed_by.trim() !== '' && typeof review.reviewed_at === 'string' && !Number.isNaN(Date.parse(review.reviewed_at))
}

if (!existsSync(manifestPath)) {
  usageError(`manifest is missing: ${manifestPath}`)
} else {
  let manifest
  try {
    manifest = JSON.parse(readFileSync(manifestPath, 'utf8'))
  } catch (error) {
    usageError(`manifest is invalid JSON: ${error.message}`)
  }

  const failures = []
  const artifactIDs = new Set()
  const scannedFiles = new Set()
  if (!manifest || !Array.isArray(manifest.artifacts)) {
    failures.push('manifest.artifacts must be an array')
  } else {
    for (const artifact of manifest.artifacts) {
      const label = artifact?.id || '<missing id>'
      if (!artifact?.id || artifactIDs.has(artifact.id)) failures.push(`${label}: id must be unique`)
      artifactIDs.add(artifact?.id)
      if (typeof artifact?.owner !== 'string' || artifact.owner.trim() === '') failures.push(`${label}: owner is required`)
      if (!Array.isArray(artifact?.paths) || artifact.paths.length === 0) failures.push(`${label}: paths are required`)
      for (const dimension of ['welfare', 'privacy']) {
        const review = artifact?.review?.[dimension]
        if (!['pending', 'approved', 'blocked'].includes(review?.status)) {
          failures.push(`${label}: ${dimension} review needs pending, approved, or blocked status`)
        }
        if (requireApproved && !reviewIsApproved(review)) {
          failures.push(`${label}: ${dimension} review must be approved with reviewer and timestamp for release`)
        }
      }
      const ownedPaths = artifact?.paths || []
      for (const declaredPath of ownedPaths) {
        const absolutePath = resolve(root, declaredPath)
        if (!underRoot(absolutePath) || !existsSync(absolutePath)) {
          failures.push(`${label}: invalid content path ${declaredPath}`)
        }
      }
      const lintPaths = artifact?.lint_paths ?? ownedPaths
      if (!Array.isArray(lintPaths)) {
        failures.push(`${label}: lint_paths must be an array when provided`)
        continue
      }
      for (const declaredPath of lintPaths) {
        const absolutePath = resolve(root, declaredPath)
        if (!underRoot(absolutePath) || !existsSync(absolutePath)) {
          failures.push(`${label}: invalid lint path ${declaredPath}`)
          continue
        }
        for (const file of filesUnder(absolutePath)) scannedFiles.add(file)
      }
    }
  }
  for (const id of requiredArtifactIDs) {
    if (!artifactIDs.has(id)) failures.push(`required artifact is not owned or reviewed: ${id}`)
  }
  for (const file of scannedFiles) {
    const text = readFileSync(file, 'utf8')
    for (const [rule, expression] of forbiddenRules) {
      if (expression.test(text)) failures.push(`${relative(root, file)}: ${rule}`)
    }
  }

  if (failures.length > 0) {
    console.error(`AP-130 narrative ethics gate failed (${failures.length}):`)
    for (const failure of failures) console.error(`- ${failure}`)
    process.exitCode = 1
  } else {
    console.log(`AP-130 narrative ethics gate OK (${scannedFiles.size} files, release approval=${requireApproved})`)
  }
}
