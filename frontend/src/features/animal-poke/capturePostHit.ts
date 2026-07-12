/**
 * AP-062: post-hit pipeline stages (hit → analyze/value → save → sync).
 * Persisted per captureAttemptId so refresh/tab switch can resume without double-reward.
 */
export type PostHitStage =
  | 'idle'
  | 'hit'
  | 'analyzing'
  | 'generating'
  | 'saving'
  | 'syncing'
  | 'completed'
  | 'saved_pending_sync'
  | 'failed'

export type PostHitTask = {
  attemptId: string
  species: string
  stage: PostHitStage
  /** stamina already consumed for this attempt */
  staminaConsumed: boolean
  /** runCaptureGeneration already succeeded (idempotent resume) */
  generated: boolean
  /** AnimalRepository.add done */
  saved: boolean
  /** sync queue flush done */
  synced: boolean
  sessionId: string | null
  valueInferenceId: string | null
  errorCode: string | null
  errorMessage: string | null
  updatedAt: number
}

const STORAGE_PREFIX = 'ap-post-hit-v1:'

export function createPostHitTask(attemptId: string, species: string): PostHitTask {
  return {
    attemptId,
    species,
    stage: 'idle',
    staminaConsumed: false,
    generated: false,
    saved: false,
    synced: false,
    sessionId: null,
    valueInferenceId: null,
    errorCode: null,
    errorMessage: null,
    updatedAt: Date.now(),
  }
}

export function storageKey(attemptId: string): string {
  return `${STORAGE_PREFIX}${attemptId}`
}

export function loadPostHitTask(attemptId: string): PostHitTask | null {
  if (typeof sessionStorage === 'undefined' || !attemptId) return null
  try {
    const raw = sessionStorage.getItem(storageKey(attemptId))
    if (!raw) return null
    const parsed = JSON.parse(raw) as PostHitTask
    if (!parsed || parsed.attemptId !== attemptId) return null
    return parsed
  } catch {
    return null
  }
}

export function savePostHitTask(task: PostHitTask): PostHitTask {
  const next = { ...task, updatedAt: Date.now() }
  try {
    if (typeof sessionStorage !== 'undefined') {
      sessionStorage.setItem(storageKey(task.attemptId), JSON.stringify(next))
    }
  } catch {
    /* ignore quota */
  }
  return next
}

export function clearPostHitTask(attemptId: string): void {
  try {
    sessionStorage.removeItem(storageKey(attemptId))
  } catch {
    /* ignore */
  }
}

/** Stages where "捕获成功" must NOT be shown */
export function isRevealAllowed(stage: PostHitStage): boolean {
  return stage === 'completed' || stage === 'saved_pending_sync'
}

export function stageLabel(stage: PostHitStage): string {
  switch (stage) {
    case 'hit':
      return '命中 · 准备生成'
    case 'analyzing':
      return '分析中…'
    case 'generating':
      return '生成属性中…'
    case 'saving':
      return '保存到图鉴…'
    case 'syncing':
      return '同步中…'
    case 'completed':
      return '捕获成功'
    case 'saved_pending_sync':
      return '已本地保存、待同步'
    case 'failed':
      return '生成失败'
    default:
      return ''
  }
}

/** Whether pipeline runner should start/resume work */
export function needsPipelineWork(task: PostHitTask): boolean {
  if (task.stage === 'completed' || task.stage === 'failed' || task.stage === 'idle') return false
  if (task.stage === 'saved_pending_sync') return !task.synced
  return true
}
