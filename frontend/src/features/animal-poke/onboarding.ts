/**
 * AP-066 event-driven onboarding.
 *
 * Task chain (only real events advance task steps):
 *   rationale → train_scan → select_target → throw → reveal → pokedex → done
 *
 * ConsentGate already collects privacy consent before the app mounts; this
 * module does not re-prompt system permissions. Denied-camera / Home Mode
 * players complete the same chain with training materials (no live animal).
 *
 * Storage is versioned so upgrades can resume or restart safely.
 */

export const ONBOARDING_STORAGE_KEY = 'animal-poke-onboarding-v2'
/** Legacy key from static multi-step overlay (AP-204 / pre-AP-066). */
const LEGACY_KEY = 'animal-poke-onboarding-v1'
export const ONBOARDING_VERSION = 2

export type OnboardingStep =
  | 'rationale'
  | 'train_scan'
  | 'select_target'
  | 'throw'
  | 'reveal'
  | 'pokedex'
  | 'done'

export type OnboardingEvent =
  | 'continue' // info-only steps (rationale)
  | 'scan_started'
  | 'detect_success'
  | 'target_selected'
  | 'target_confirmed'
  | 'throw_started'
  | 'capture_success'
  | 'capture_failed'
  | 'pokedex_opened'
  | 'skip'
  | 'reset'

export interface OnboardingState {
  version: number
  step: OnboardingStep
  skipped: boolean
  completedAt: number | null
  /** First training capture must not feed formal economy / leaderboards. */
  trainingCaptureDone: boolean
  /** True while player is mid-tutorial (not done/skipped). */
  active: boolean
  /** Optional path flags for analytics / resume UX. */
  path: 'outdoor' | 'home_training'
  updatedAt: number
}

const ORDER: OnboardingStep[] = [
  'rationale',
  'train_scan',
  'select_target',
  'throw',
  'reveal',
  'pokedex',
  'done',
]

/**
 * Steps that only advance on `continue` and use a full-screen modal.
 * `reveal` is a non-blocking coach so the player can still open 图鉴 via tabs.
 */
const INFO_STEPS: OnboardingStep[] = ['rationale']
const MODAL_STEPS: OnboardingStep[] = ['rationale']

const INITIAL: OnboardingState = {
  version: ONBOARDING_VERSION,
  step: 'rationale',
  skipped: false,
  completedAt: null,
  trainingCaptureDone: false,
  active: true,
  path: 'outdoor',
  updatedAt: 0,
}

export function isOnboardingActive(s: OnboardingState = loadOnboarding()): boolean {
  return s.active && s.step !== 'done' && !s.skipped
}

/** Camera / geolocation must not start until the player leaves rationale. */
export function shouldDeferSensors(s: OnboardingState = loadOnboarding()): boolean {
  return isOnboardingActive(s) && s.step === 'rationale'
}

/** First tutorial capture stays out of formal economy / ranking. */
export function isTrainingCapture(s: OnboardingState = loadOnboarding()): boolean {
  return isOnboardingActive(s) || (s.trainingCaptureDone && s.step === 'done' && !s.skipped && s.completedAt != null && Date.now() - s.completedAt < 60_000)
}

export function shouldBlockEconomyForCapture(s: OnboardingState = loadOnboarding()): boolean {
  // While tutorial is active OR the capture that completed training is settling.
  return (
    isOnboardingActive(s) &&
    (s.step === 'throw' || s.step === 'reveal' || s.step === 'select_target' || s.step === 'train_scan')
  )
}

export function nextStep(step: OnboardingStep): OnboardingStep {
  const i = ORDER.indexOf(step)
  if (i < 0) return 'done'
  return ORDER[Math.min(ORDER.length - 1, i + 1)]
}

function touch(partial: Partial<OnboardingState> & Pick<OnboardingState, 'step'>): OnboardingState {
  const cur = loadOnboarding()
  const step = partial.step
  const done = step === 'done'
  return {
    ...cur,
    ...partial,
    version: ONBOARDING_VERSION,
    active: !done && !(partial.skipped ?? cur.skipped),
    completedAt: done ? partial.completedAt ?? Date.now() : null,
    updatedAt: Date.now(),
  }
}

function migrateLegacy(): OnboardingState | null {
  try {
    const raw = localStorage.getItem(LEGACY_KEY)
    if (!raw) return null
    const legacy = JSON.parse(raw) as { step?: string; skipped?: boolean; completedAt?: number | null }
    // Completed / skipped legacy → treat as done so we don't re-trap veterans.
    if (legacy.skipped || legacy.step === 'done' || legacy.completedAt) {
      return {
        ...INITIAL,
        step: 'done',
        skipped: !!legacy.skipped,
        completedAt: legacy.completedAt ?? Date.now(),
        active: false,
        trainingCaptureDone: true,
        updatedAt: Date.now(),
      }
    }
    // Mid-flow legacy static steps map into the nearest event-driven step.
    const map: Record<string, OnboardingStep> = {
      welcome: 'rationale',
      permissions: 'rationale',
      scan_tip: 'train_scan',
      throw_tip: 'throw',
      pokedex_tip: 'pokedex',
    }
    const step = map[legacy.step || ''] || 'rationale'
    return {
      ...INITIAL,
      step,
      active: step !== 'done',
      updatedAt: Date.now(),
    }
  } catch {
    return null
  }
}

export function loadOnboarding(): OnboardingState {
  try {
    const raw = localStorage.getItem(ONBOARDING_STORAGE_KEY)
    if (raw) {
      const parsed = JSON.parse(raw) as Partial<OnboardingState>
      if (parsed && typeof parsed === 'object' && parsed.version === ONBOARDING_VERSION && parsed.step) {
        return {
          ...INITIAL,
          ...parsed,
          version: ONBOARDING_VERSION,
          active: parsed.step !== 'done' && !parsed.skipped,
        } as OnboardingState
      }
      // Version mismatch → restart tutorial (upgrade path) but keep skip if done.
      if (parsed?.step === 'done' || parsed?.skipped) {
        return {
          ...INITIAL,
          step: 'done',
          skipped: !!parsed.skipped,
          completedAt: parsed.completedAt ?? Date.now(),
          active: false,
          trainingCaptureDone: true,
          updatedAt: Date.now(),
        }
      }
    }
    const legacy = migrateLegacy()
    if (legacy) {
      saveOnboarding(legacy)
      try {
        localStorage.removeItem(LEGACY_KEY)
      } catch {
        /* ignore */
      }
      return legacy
    }
    return { ...INITIAL, updatedAt: Date.now() }
  } catch {
    return { ...INITIAL, updatedAt: Date.now() }
  }
}

export function saveOnboarding(s: OnboardingState): void {
  try {
    localStorage.setItem(ONBOARDING_STORAGE_KEY, JSON.stringify(s))
  } catch {
    /* ignore */
  }
}

/**
 * Pure event reducer. Duplicate events are idempotent (no step regression).
 * Returns the next state (caller persists via saveOnboarding / applyOnboardingEvent).
 */
export function reduceOnboardingEvent(
  state: OnboardingState,
  event: OnboardingEvent,
  opts?: { path?: 'outdoor' | 'home_training' },
): OnboardingState {
  if (event === 'reset') {
    return { ...INITIAL, updatedAt: Date.now() }
  }
  if (event === 'skip') {
    return touch({
      step: 'done',
      skipped: true,
      completedAt: Date.now(),
      active: false,
      trainingCaptureDone: state.trainingCaptureDone,
    })
  }

  if (state.step === 'done' || state.skipped) {
    return state
  }

  const withPath = opts?.path ? { path: opts.path } : {}

  switch (state.step) {
    case 'rationale':
      if (event === 'continue') {
        return touch({ step: 'train_scan', ...withPath })
      }
      return state

    case 'train_scan':
      // Advance on successful detect (or scan_started only keeps coach copy).
      if (event === 'detect_success') {
        return touch({ step: 'select_target', ...withPath })
      }
      if (event === 'scan_started') {
        return { ...state, ...withPath, updatedAt: Date.now() }
      }
      return state

    case 'select_target':
      if (event === 'target_selected' || event === 'target_confirmed') {
        // Single-target auto-confirm also emits target_confirmed.
        return touch({
          step: event === 'target_confirmed' ? 'throw' : 'select_target',
          ...withPath,
        })
      }
      // Re-scan allowed without regressing.
      if (event === 'detect_success' || event === 'scan_started') {
        return { ...state, updatedAt: Date.now() }
      }
      return state

    case 'throw':
      if (event === 'throw_started') {
        return { ...state, updatedAt: Date.now() }
      }
      if (event === 'capture_success') {
        return touch({
          step: 'reveal',
          trainingCaptureDone: true,
          ...withPath,
        })
      }
      if (event === 'capture_failed') {
        // Stay on throw — retry is the lesson.
        return { ...state, updatedAt: Date.now() }
      }
      return state

    case 'reveal':
      if (event === 'continue' || event === 'pokedex_opened') {
        return touch({
          step: event === 'pokedex_opened' ? 'done' : 'pokedex',
          trainingCaptureDone: true,
          ...withPath,
        })
      }
      return state

    case 'pokedex':
      if (event === 'pokedex_opened') {
        return touch({
          step: 'done',
          trainingCaptureDone: true,
          completedAt: Date.now(),
          ...withPath,
        })
      }
      return state

    default:
      return state
  }
}

/** Apply event, persist, return new state. */
export function applyOnboardingEvent(
  event: OnboardingEvent,
  opts?: { path?: 'outdoor' | 'home_training' },
): OnboardingState {
  const cur = loadOnboarding()
  const next = reduceOnboardingEvent(cur, event, opts)
  if (next !== cur) saveOnboarding(next)
  else if (next.updatedAt !== cur.updatedAt) saveOnboarding(next)
  return next
}

/** @deprecated Use applyOnboardingEvent('continue') — kept for Settings replay tests. */
export function advanceOnboarding(): OnboardingState {
  return applyOnboardingEvent('continue')
}

export function skipOnboarding(): OnboardingState {
  return applyOnboardingEvent('skip')
}

export function resetOnboarding(): OnboardingState {
  const next = reduceOnboardingEvent(loadOnboarding(), 'reset')
  saveOnboarding(next)
  try {
    localStorage.removeItem(LEGACY_KEY)
  } catch {
    /* ignore */
  }
  if (typeof window !== 'undefined') {
    window.dispatchEvent(new CustomEvent('animal-poke-onboarding-changed', { detail: next }))
  }
  return next
}

export function stepCopy(step: OnboardingStep): { title: string; body: string; waitHint?: string } {
  switch (step) {
    case 'rationale':
      return {
        title: '欢迎来到 Animal Poke',
        body: '用镜头温柔观察身边的小伙伴，生成你的手账图鉴。相机与定位只在你真正开始发现时才会请求。',
        waitHint: '点「开始训练」进入下一步',
      }
    case 'train_scan':
      return {
        title: '第一次扫描',
        body: '对准主体，光线充足、画面稳定后点「开始识别」。没有真实动物时会使用训练素材。',
        waitHint: '完成一次识别后自动继续',
      }
    case 'select_target':
      return {
        title: '选择目标',
        body: '点选画面中的目标框（或多个候选中更清晰的一只），再进入捕获。',
        waitHint: '确认目标后自动继续',
      }
    case 'throw':
      return {
        title: '投掷捕获',
        body: '按住蓄力、松开投掷。失败可再试；本次训练不计入正式经济与排行。',
        waitHint: '完成一次捕获后自动继续',
      }
    case 'reveal':
      return {
        title: '收藏揭晓',
        body: '训练捕获已保存到本地图鉴。去图鉴看看你的第一张贴纸吧！',
        waitHint: '点「去图鉴」或自行打开图鉴页',
      }
    case 'pokedex':
      return {
        title: '打开图鉴',
        body: '在底部导航打开「图鉴」，查看详情与研究笔记。',
        waitHint: '打开图鉴后教学完成',
      }
    default:
      return { title: '准备就绪', body: '开始你的城市探索吧！' }
  }
}

export function stepOrder(): readonly OnboardingStep[] {
  return ORDER
}

export function isInfoStep(step: OnboardingStep): boolean {
  return INFO_STEPS.includes(step)
}

/** Full-screen modal only for rationale (before sensors start). */
export function isModalStep(step: OnboardingStep): boolean {
  return MODAL_STEPS.includes(step)
}
