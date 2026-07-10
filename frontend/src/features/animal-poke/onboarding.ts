/** 首 10 分钟引导状态（#204） */
const KEY = 'animal-poke-onboarding-v1'

export type OnboardingStep =
  | 'welcome'
  | 'permissions'
  | 'scan_tip'
  | 'throw_tip'
  | 'pokedex_tip'
  | 'done'

export interface OnboardingState {
  step: OnboardingStep
  skipped: boolean
  completedAt: number | null
}

const ORDER: OnboardingStep[] = [
  'welcome',
  'permissions',
  'scan_tip',
  'throw_tip',
  'pokedex_tip',
  'done',
]

export function loadOnboarding(): OnboardingState {
  try {
    const raw = localStorage.getItem(KEY)
    if (!raw) return { step: 'welcome', skipped: false, completedAt: null }
    return JSON.parse(raw) as OnboardingState
  } catch {
    return { step: 'welcome', skipped: false, completedAt: null }
  }
}

export function saveOnboarding(s: OnboardingState): void {
  try {
    localStorage.setItem(KEY, JSON.stringify(s))
  } catch {
    /* ignore */
  }
}

export function nextStep(step: OnboardingStep): OnboardingStep {
  const i = ORDER.indexOf(step)
  return ORDER[Math.min(ORDER.length - 1, i + 1)]
}

export function advanceOnboarding(): OnboardingState {
  const cur = loadOnboarding()
  if (cur.step === 'done' || cur.skipped) return cur
  const step = nextStep(cur.step)
  const next: OnboardingState = {
    step,
    skipped: false,
    completedAt: step === 'done' ? Date.now() : null,
  }
  saveOnboarding(next)
  return next
}

export function skipOnboarding(): OnboardingState {
  const next: OnboardingState = { step: 'done', skipped: true, completedAt: Date.now() }
  saveOnboarding(next)
  return next
}

export function resetOnboarding(): OnboardingState {
  const next: OnboardingState = { step: 'welcome', skipped: false, completedAt: null }
  saveOnboarding(next)
  return next
}

export function stepCopy(step: OnboardingStep): { title: string; body: string } {
  switch (step) {
    case 'welcome':
      return { title: '欢迎来到 AnimalPoke', body: '用相机温柔观察附近的猫狗鹅，生成你的手账图鉴。' }
    case 'permissions':
      return {
        title: '权限说明',
        body: '相机用于识别，位置用于天气与发现点。可拒绝，但发现/捕获将不可用，仍可浏览图鉴。',
      }
    case 'scan_tip':
      return { title: '扫描技巧', body: '靠近、光线充足、主体完整。本版本是手动扫描，不是无限实时上传。' }
    case 'throw_tip':
      return { title: '投掷教学', body: '按住蓄力、松开投掷。失败可再试，体力按 attempt 扣除。' }
    case 'pokedex_tip':
      return { title: '图鉴价值', body: '首次捕获解锁贴纸；去图鉴查看详情。' }
    default:
      return { title: '准备就绪', body: '开始你的第一次发现吧！' }
  }
}
