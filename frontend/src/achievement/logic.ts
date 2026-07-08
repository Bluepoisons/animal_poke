import { ACHIEVEMENT_DEFS, ACHIEVEMENT_MAP } from './constants'
import type {
  AchievementDef,
  AchievementStats,
  AchievementProgress,
  AchievementCheckResult,
  UnlockedAchievement,
} from './types'

/**
 * 根据成就条件和统计数据，判定单个成就是否应解锁
 */
export function isAchievementUnlocked(def: AchievementDef, stats: AchievementStats): boolean {
  const { condition } = def
  switch (condition.type) {
    case 'total_captures':
      return stats.totalCaptures >= condition.target
    case 'captures_by_rarity':
      return stats.capturesByRarity[condition.rarity] >= condition.target
    case 'captures_by_species':
      return stats.capturesBySpecies[condition.species] >= condition.target
    case 'all_species_diversity':
      return stats.capturesBySpecies.cat >= condition.target
        && stats.capturesBySpecies.goose >= condition.target
        && stats.capturesBySpecies.dog >= condition.target
    case 'total_battles_won':
      return stats.totalBattlesWon >= condition.target
    case 'total_battles':
      return stats.totalBattles >= condition.target
    case 'win_streak':
      return stats.maxWinStreak >= condition.target
    case 'level_reached':
      return stats.level >= condition.target
    case 'check_in_days':
      return stats.checkInStreak >= condition.target
    case 'cities_visited':
      return stats.citiesVisited >= condition.target
    case 'weather_types_experienced':
      return stats.weatherTypesExperienced.length >= condition.target
    case 'rain_captures_no_cold':
      return stats.rainCapturesNoCold >= condition.target
    case 'legendary_captured':
      return stats.hasLegendary
    case 'all_weather_types':
      return stats.weatherTypesExperienced.length >= 7
    default:
      return false
  }
}

/**
 * 获取单个成就的当前进度值
 */
export function getAchievementCurrent(def: AchievementDef, stats: AchievementStats): number {
  const { condition } = def
  switch (condition.type) {
    case 'total_captures':
      return stats.totalCaptures
    case 'captures_by_rarity':
      return stats.capturesByRarity[condition.rarity]
    case 'captures_by_species':
      return stats.capturesBySpecies[condition.species]
    case 'all_species_diversity':
      return Math.min(
        stats.capturesBySpecies.cat,
        stats.capturesBySpecies.goose,
        stats.capturesBySpecies.dog
      )
    case 'total_battles_won':
      return stats.totalBattlesWon
    case 'total_battles':
      return stats.totalBattles
    case 'win_streak':
      return stats.maxWinStreak
    case 'level_reached':
      return stats.level
    case 'check_in_days':
      return stats.checkInStreak
    case 'cities_visited':
      return stats.citiesVisited
    case 'weather_types_experienced':
      return stats.weatherTypesExperienced.length
    case 'rain_captures_no_cold':
      return stats.rainCapturesNoCold
    case 'legendary_captured':
      return stats.hasLegendary ? 1 : 0
    case 'all_weather_types':
      return stats.weatherTypesExperienced.length
    default:
      return 0
  }
}

/**
 * 批量检查所有成就，返回新解锁的成就 ID 列表
 */
export function checkAchievements(
  stats: AchievementStats,
  alreadyUnlocked: Set<string>
): AchievementCheckResult {
  const newlyUnlocked: string[] = []

  for (const def of ACHIEVEMENT_DEFS) {
    if (alreadyUnlocked.has(def.id)) continue
    if (isAchievementUnlocked(def, stats)) {
      newlyUnlocked.push(def.id)
    }
  }

  return {
    newlyUnlocked,
    changed: newlyUnlocked.length > 0,
  }
}

/**
 * 获取单个成就的完整进度信息
 */
export function getAchievementProgress(
  def: AchievementDef,
  stats: AchievementStats,
  unlocked: boolean
): AchievementProgress {
  const current = getAchievementCurrent(def, stats)
  const target = def.target
  const percent = target > 0 ? Math.min(100, Math.round((current / target) * 100)) : 0

  return {
    id: def.id,
    current,
    target,
    unlocked,
    percent: unlocked ? 100 : percent,
  }
}

/**
 * 获取所有成就的进度列表
 */
export function getAllAchievementProgress(
  stats: AchievementStats,
  unlockedIds: Set<string>
): AchievementProgress[] {
  return ACHIEVEMENT_DEFS.map((def) =>
    getAchievementProgress(def, stats, unlockedIds.has(def.id))
  )
}

/**
 * 从成就定义中提取所有可获得的称号
 */
export function getAllTitles(): string[] {
  const titles: string[] = []
  for (const def of ACHIEVEMENT_DEFS) {
    if (def.reward.title) {
      titles.push(def.reward.title)
    }
  }
  return titles
}

/**
 * 从已解锁成就中提取已获得的称号
 */
export function getUnlockedTitles(unlocked: UnlockedAchievement[]): string[] {
  const titles: string[] = []
  const unlockedIds = new Set(unlocked.map((u) => u.id))
  for (const def of ACHIEVEMENT_DEFS) {
    if (unlockedIds.has(def.id) && def.reward.title) {
      titles.push(def.reward.title)
    }
  }
  return titles
}

/**
 * 计算成就完成率
 */
export function getCompletionRate(unlockedCount: number): number {
  const total = ACHIEVEMENT_DEFS.length
  return total > 0 ? Math.round((unlockedCount / total) * 100) : 0
}

/**
 * 计算成就奖励金币总数（已解锁的）
 */
export function getTotalRewardGold(unlockedIds: Set<string>): number {
  let total = 0
  for (const def of ACHIEVEMENT_DEFS) {
    if (unlockedIds.has(def.id)) {
      total += def.reward.gold
    }
  }
  return total
}
