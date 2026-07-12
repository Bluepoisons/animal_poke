/**
 * AP-109 Home Mode — equivalent progress without camera/location/outdoor mobility.
 * Reward budget matches outdoor path so Home Mode is not a farm shortcut.
 */
export type ActivityKind =
  | 'first_day_capture_or_train'
  | 'level_up_action'
  | 'daily_goal'
  | 'narrative_beat'
  | 'dispatch'

/** Gold (or equivalent) budget per activity — same for outdoor vs home */
export const ACTIVITY_REWARD_BUDGET: Record<ActivityKind, number> = {
  first_day_capture_or_train: 30,
  level_up_action: 20,
  daily_goal: 10,
  narrative_beat: 8,
  dispatch: 15,
}

export type HomeActivity = {
  id: string
  title: string
  description: string
  kind: ActivityKind
  /** Never requires camera */
  needsCamera: false
  /** Never requires GPS */
  needsLocation: false
  reward: number
}

export const HOME_ACTIVITIES: HomeActivity[] = [
  {
    id: 'home.train_observe',
    title: '训练观察',
    description: '使用授权公共/合成素材完成一次观察记录',
    kind: 'first_day_capture_or_train',
    needsCamera: false,
    needsLocation: false,
    reward: ACTIVITY_REWARD_BUDGET.first_day_capture_or_train,
  },
  {
    id: 'home.research_owned',
    title: '已收藏研究',
    description: '为已有收藏补充笔记（等价于一次安全探索进度）',
    kind: 'daily_goal',
    needsCamera: false,
    needsLocation: false,
    reward: ACTIVITY_REWARD_BUDGET.daily_goal,
  },
  {
    id: 'home.knowledge_task',
    title: '知识任务',
    description: '完成一条城市知识问答（不绑定行动能力）',
    kind: 'level_up_action',
    needsCamera: false,
    needsLocation: false,
    reward: ACTIVITY_REWARD_BUDGET.level_up_action,
  },
  {
    id: 'home.virtual_route',
    title: '虚拟路线',
    description: '在地图上走完一条室内/桌面虚拟路线',
    kind: 'daily_goal',
    needsCamera: false,
    needsLocation: false,
    reward: ACTIVITY_REWARD_BUDGET.daily_goal,
  },
  {
    id: 'home.dispatch',
    title: '派遣整理',
    description: '整理派遣回报（与户外派遣同预算）',
    kind: 'dispatch',
    needsCamera: false,
    needsLocation: false,
    reward: ACTIVITY_REWARD_BUDGET.dispatch,
  },
  {
    id: 'home.narrative',
    title: '手账章节',
    description: '推进叙事节拍（序章/热线路径）',
    kind: 'narrative_beat',
    needsCamera: false,
    needsLocation: false,
    reward: ACTIVITY_REWARD_BUDGET.narrative_beat,
  },
]

export function rewardFor(kind: ActivityKind, homeMode: boolean): number {
  // Explicitly equal — homeMode flag must not inflate reward
  void homeMode
  return ACTIVITY_REWARD_BUDGET[kind]
}

export function isHomeModeEquivalent(outdoorReward: number, homeReward: number): boolean {
  return outdoorReward === homeReward
}

export function d1HomeUnlocks(): string[] {
  return ['home.train_observe', 'home.research_owned', 'home.narrative']
}

export function d7HomeUnlocks(): string[] {
  return ['home.knowledge_task', 'home.virtual_route', 'home.dispatch']
}

export function validateHomeModeParity(): string[] {
  const errors: string[] = []
  for (const a of HOME_ACTIVITIES) {
    if (a.needsCamera || a.needsLocation) errors.push(`${a.id} must not need camera/location`)
    if (a.reward !== ACTIVITY_REWARD_BUDGET[a.kind]) errors.push(`${a.id} reward mismatch budget`)
    if (rewardFor(a.kind, true) !== rewardFor(a.kind, false)) {
      errors.push(`${a.id} home/outdoor reward not equal`)
    }
  }
  if (d1HomeUnlocks().length < 3) errors.push('D1 needs ≥3 home alternatives')
  if (d7HomeUnlocks().length < 3) errors.push('D7 needs ≥3 home alternatives')
  return errors
}
