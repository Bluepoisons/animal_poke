import type { FeatureUnlockGate, GoalDef } from './types'

/** localStorage key */
export const PROGRESSION_STORAGE_KEY = 'animal_poke_progression'

/** Days away before treating as returning player (no FOMO) */
export const RETURN_THRESHOLD_DAYS = 3

/** Always show this many executable goals on the home surface */
export const EXECUTABLE_GOAL_COUNT = 3

/** ms in one calendar day (UTC day boundaries used for dayIndex) */
export const MS_PER_DAY = 86_400_000

/**
 * D1 / D7 / D30 / daily goal catalog.
 * D1 trio is always free-or-basic so day-1 always has 3 executable goals;
 * when stamina is 0, free goals remain available.
 */
export const GOAL_DEFS: GoalDef[] = [
  // ===== D1 (day index 0) — first capture, pokedex, free safe explore =====
  {
    id: 'd1_first_capture',
    horizon: 'D1',
    title: '完成首次捕获',
    description: '用相机识别并收藏你的第一只动物',
    action: 'capture',
    target: 1,
    free: false,
    navigateTo: 'discover',
    rewardGold: 30,
    minLevel: 1,
    minCaptures: 0,
    minDayIndex: 0,
  },
  {
    id: 'd1_open_pokedex',
    horizon: 'D1',
    title: '打开图鉴',
    description: '浏览一次图鉴，看看已收集与待发现',
    action: 'open_pokedex',
    target: 1,
    free: true,
    navigateTo: 'pokedex',
    rewardGold: 10,
    minLevel: 1,
    minCaptures: 0,
    minDayIndex: 0,
  },
  {
    id: 'd1_safe_explore',
    horizon: 'D1',
    title: '安全探索',
    description: '不消耗体力，完成一次安全探索指引',
    action: 'safe_explore',
    target: 1,
    free: true,
    navigateTo: 'map',
    rewardGold: 10,
    minLevel: 1,
    minCaptures: 0,
    minDayIndex: 0,
  },

  // ===== D7 (day index >= 6) — species diversity, battle, dispatch =====
  {
    id: 'd7_three_species',
    horizon: 'D7',
    title: '收集 3 个物种',
    description: '至少收藏猫 / 狗 / 鹅中的 3 种',
    action: 'collect_species',
    target: 3,
    free: false,
    navigateTo: 'discover',
    rewardGold: 80,
    minLevel: 1,
    minCaptures: 0,
    minDayIndex: 0,
  },
  {
    id: 'd7_first_battle',
    horizon: 'D7',
    title: '完成一场战斗',
    description: '解锁战斗后完成 1 场对战',
    action: 'battle',
    target: 1,
    free: false,
    navigateTo: 'battle',
    rewardGold: 50,
    minLevel: 2,
    minCaptures: 1,
    minDayIndex: 1,
  },
  {
    id: 'd7_first_dispatch',
    horizon: 'D7',
    title: '完成一次派遣',
    description: '派宠物外出探索并领取奖励',
    action: 'dispatch',
    target: 1,
    free: false,
    navigateTo: 'dispatch',
    rewardGold: 50,
    minLevel: 3,
    minCaptures: 3,
    minDayIndex: 3,
  },

  // ===== D30 — region / season long-term =====
  {
    id: 'd30_three_regions',
    horizon: 'D30',
    title: '造访 3 个区域',
    description: '在不同城市完成发现或捕获',
    action: 'visit_region',
    target: 3,
    free: true,
    navigateTo: 'map',
    rewardGold: 120,
    minLevel: 1,
    minCaptures: 0,
    minDayIndex: 7,
  },
  {
    id: 'd30_season_checkins',
    horizon: 'D30',
    title: '赛季签到 7 次',
    description: '长期赛季节奏，完成 7 次赛季签到',
    action: 'season_checkin',
    target: 7,
    free: true,
    navigateTo: 'discover',
    rewardGold: 150,
    minLevel: 3,
    minCaptures: 0,
    minDayIndex: 7,
  },
  {
    id: 'd30_collection_20',
    horizon: 'D30',
    title: '收藏 20 只动物',
    description: '长期收集目标，稳步扩充图鉴',
    action: 'capture',
    target: 20,
    free: false,
    navigateTo: 'discover',
    rewardGold: 200,
    minLevel: 1,
    minCaptures: 0,
    minDayIndex: 7,
  },

  // ===== Daily pool (rotated / filled when horizon goals done) =====
  {
    id: 'daily_capture',
    horizon: 'daily',
    title: '今日捕获 1 次',
    description: '完成一次捕获（消耗体力）',
    action: 'capture',
    target: 1,
    free: false,
    navigateTo: 'discover',
    rewardGold: 15,
    minLevel: 1,
    minCaptures: 0,
    minDayIndex: 0,
  },
  {
    id: 'daily_pokedex',
    horizon: 'daily',
    title: '今日浏览图鉴',
    description: '打开图鉴一次（不耗体力）',
    action: 'open_pokedex',
    target: 1,
    free: true,
    navigateTo: 'pokedex',
    rewardGold: 5,
    minLevel: 1,
    minCaptures: 0,
    minDayIndex: 0,
  },
  {
    id: 'daily_safe_explore',
    horizon: 'daily',
    title: '今日安全探索',
    description: '体力耗尽时仍可做的免费探索',
    action: 'safe_explore',
    target: 1,
    free: true,
    navigateTo: 'map',
    rewardGold: 5,
    minLevel: 1,
    minCaptures: 0,
    minDayIndex: 0,
  },
  {
    id: 'daily_battle',
    horizon: 'daily',
    title: '今日战斗 1 场',
    description: '完成一场战斗',
    action: 'battle',
    target: 1,
    free: false,
    navigateTo: 'battle',
    rewardGold: 20,
    minLevel: 2,
    minCaptures: 1,
    minDayIndex: 1,
  },
  {
    id: 'daily_season',
    horizon: 'daily',
    title: '赛季签到',
    description: '完成今日赛季签到（免费）',
    action: 'season_checkin',
    target: 1,
    free: true,
    navigateTo: 'discover',
    rewardGold: 8,
    minLevel: 3,
    minCaptures: 0,
    minDayIndex: 7,
  },
]

export const GOAL_MAP: Record<string, GoalDef> = Object.fromEntries(
  GOAL_DEFS.map((g) => [g.id, g]),
)

/** Feature unlock gates — locked features are hidden, never toast-spammed */
export const FEATURE_GATES: FeatureUnlockGate[] = [
  { feature: 'discover', minLevel: 1, minCaptures: 0, minDayIndex: 0, minBattles: 0, minSpecies: 0, always: true },
  { feature: 'map', minLevel: 1, minCaptures: 0, minDayIndex: 0, minBattles: 0, minSpecies: 0, always: true },
  { feature: 'pokedex', minLevel: 1, minCaptures: 0, minDayIndex: 0, minBattles: 0, minSpecies: 0, always: true },
  { feature: 'store', minLevel: 1, minCaptures: 0, minDayIndex: 0, minBattles: 0, minSpecies: 0, always: true },
  // battle after first capture / level 2 (D1→D7 bridge)
  { feature: 'battle', minLevel: 2, minCaptures: 1, minDayIndex: 0, minBattles: 0, minSpecies: 0 },
  // achievement after first capture
  { feature: 'achievement', minLevel: 2, minCaptures: 1, minDayIndex: 0, minBattles: 0, minSpecies: 0 },
  // dispatch around D7
  { feature: 'dispatch', minLevel: 3, minCaptures: 3, minDayIndex: 3, minBattles: 0, minSpecies: 0 },
  // season long-term (D7+)
  { feature: 'season', minLevel: 3, minCaptures: 0, minDayIndex: 7, minBattles: 0, minSpecies: 0 },
  // pvp later
  { feature: 'pvp', minLevel: 5, minCaptures: 5, minDayIndex: 14, minBattles: 3, minSpecies: 0 },
]

export const FEATURE_GATE_MAP: Record<string, FeatureUnlockGate> = Object.fromEntries(
  FEATURE_GATES.map((g) => [g.feature, g]),
)

/** Tab order for production BottomTabBar (achievement is soft-link) */
export const TAB_FEATURE_ORDER: Array<'discover' | 'pokedex' | 'battle' | 'store' | 'achievement'> = [
  'discover',
  'pokedex',
  'battle',
  'store',
  'achievement',
]
