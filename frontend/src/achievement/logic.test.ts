import { describe, it, expect } from 'vitest'
import {
  isAchievementUnlocked,
  getAchievementCurrent,
  checkAchievements,
  getAchievementProgress,
  getAllAchievementProgress,
  getUnlockedTitles,
  getCompletionRate,
  getTotalRewardGold,
} from './logic'
import { ACHIEVEMENT_DEFS, ACHIEVEMENT_MAP } from './constants'
import type { AchievementStats } from './types'

// 构建测试用统计数据
function makeStats(overrides: Partial<AchievementStats> = {}): AchievementStats {
  return {
    totalCaptures: 0,
    totalBattlesWon: 0,
    totalBattles: 0,
    currentWinStreak: 0,
    maxWinStreak: 0,
    level: 1,
    checkInStreak: 0,
    citiesVisited: 0,
    weatherTypesExperienced: [],
    capturesByRarity: { common: 0, uncommon: 0, rare: 0, epic: 0, legendary: 0 },
    capturesBySpecies: { cat: 0, goose: 0, dog: 0 },
    hasLegendary: false,
    rainCapturesNoCold: 0,
    ...overrides,
  }
}

describe('isAchievementUnlocked', () => {
  it('#1 total_captures=1 解锁 first_capture', () => {
    const def = ACHIEVEMENT_MAP['first_capture']
    const stats = makeStats({ totalCaptures: 1 })
    expect(isAchievementUnlocked(def, stats)).toBe(true)
  })

  it('#2 total_captures=0 未解锁 first_capture', () => {
    const def = ACHIEVEMENT_MAP['first_capture']
    const stats = makeStats({ totalCaptures: 0 })
    expect(isAchievementUnlocked(def, stats)).toBe(false)
  })

  it('#3 captures_by_rarity rare=10 解锁 rare_collector_10', () => {
    const def = ACHIEVEMENT_MAP['rare_collector_10']
    const stats = makeStats({
      capturesByRarity: { common: 5, uncommon: 3, rare: 10, epic: 0, legendary: 0 },
    })
    expect(isAchievementUnlocked(def, stats)).toBe(true)
  })

  it('#4 captures_by_species cat=10 解锁 cat_lover_10', () => {
    const def = ACHIEVEMENT_MAP['cat_lover_10']
    const stats = makeStats({
      capturesBySpecies: { cat: 10, goose: 2, dog: 3 },
    })
    expect(isAchievementUnlocked(def, stats)).toBe(true)
  })

  it('#5 level=5 解锁 level_5', () => {
    const def = ACHIEVEMENT_MAP['level_5']
    const stats = makeStats({ level: 5 })
    expect(isAchievementUnlocked(def, stats)).toBe(true)
  })

  it('#6 level=4 未解锁 level_5', () => {
    const def = ACHIEVEMENT_MAP['level_5']
    const stats = makeStats({ level: 4 })
    expect(isAchievementUnlocked(def, stats)).toBe(false)
  })

  it('#7 hasLegendary=true 解锁 legendary_captured', () => {
    const def = ACHIEVEMENT_MAP['legendary_captured']
    const stats = makeStats({ hasLegendary: true })
    expect(isAchievementUnlocked(def, stats)).toBe(true)
  })

  it('#8 all_weather_types 7种天气解锁 weather_master', () => {
    const def = ACHIEVEMENT_MAP['weather_master']
    const stats = makeStats({
      weatherTypesExperienced: ['sunny', 'cloudy', 'overcast', 'rainy', 'snowy', 'foggy', 'extreme'],
    })
    expect(isAchievementUnlocked(def, stats)).toBe(true)
  })

  it('#9 win_streak=5 解锁 win_streak_5', () => {
    const def = ACHIEVEMENT_MAP['win_streak_5']
    const stats = makeStats({ maxWinStreak: 5 })
    expect(isAchievementUnlocked(def, stats)).toBe(true)
  })

  it('#10 win_streak=4 未解锁 win_streak_5', () => {
    const def = ACHIEVEMENT_MAP['win_streak_5']
    const stats = makeStats({ maxWinStreak: 4 })
    expect(isAchievementUnlocked(def, stats)).toBe(false)
  })
})

describe('getAchievementCurrent', () => {
  it('#11 total_captures=35 时 collector_50 进度=35', () => {
    const def = ACHIEVEMENT_MAP['collector_50']
    const stats = makeStats({ totalCaptures: 35 })
    expect(getAchievementCurrent(def, stats)).toBe(35)
  })

  it('#12 legendary_captured 未捕获时进度=0', () => {
    const def = ACHIEVEMENT_MAP['legendary_captured']
    const stats = makeStats({ hasLegendary: false })
    expect(getAchievementCurrent(def, stats)).toBe(0)
  })

  it('#13 weather_master 3种天气时进度=3', () => {
    const def = ACHIEVEMENT_MAP['weather_master']
    const stats = makeStats({
      weatherTypesExperienced: ['sunny', 'rainy', 'snowy'],
    })
    expect(getAchievementCurrent(def, stats)).toBe(3)
  })
})

describe('checkAchievements', () => {
  it('#14 首次捕获解锁 first_capture', () => {
    const stats = makeStats({ totalCaptures: 1 })
    const alreadyUnlocked = new Set<string>()
    const result = checkAchievements(stats, alreadyUnlocked)
    expect(result.changed).toBe(true)
    expect(result.newlyUnlocked).toContain('first_capture')
  })

  it('#15 已解锁的成就不重复解锁', () => {
    const stats = makeStats({ totalCaptures: 1 })
    const alreadyUnlocked = new Set(['first_capture'])
    const result = checkAchievements(stats, alreadyUnlocked)
    expect(result.newlyUnlocked).not.toContain('first_capture')
  })

  it('#16 一次满足多个条件时批量解锁', () => {
    const stats = makeStats({
      totalCaptures: 50,
      totalBattlesWon: 1,
      level: 5,
    })
    const alreadyUnlocked = new Set<string>()
    const result = checkAchievements(stats, alreadyUnlocked)
    expect(result.newlyUnlocked.length).toBeGreaterThanOrEqual(3)
    expect(result.newlyUnlocked).toContain('collector_50')
    expect(result.newlyUnlocked).toContain('first_battle_win')
    expect(result.newlyUnlocked).toContain('level_5')
  })

  it('#17 无新成就时 changed=false', () => {
    const stats = makeStats({ totalCaptures: 0 })
    // guild_member has target=0 for cities_visited, so it's always unlocked;
    // pre-populate it to simulate already-unlocked state
    const alreadyUnlocked = new Set(['guild_member'])
    const result = checkAchievements(stats, alreadyUnlocked)
    expect(result.changed).toBe(false)
    expect(result.newlyUnlocked).toEqual([])
  })
})

describe('getAchievementProgress', () => {
  it('#18 未解锁成就 percent=50', () => {
    const def = ACHIEVEMENT_MAP['collector_50']
    const stats = makeStats({ totalCaptures: 25 })
    const progress = getAchievementProgress(def, stats, false)
    expect(progress.current).toBe(25)
    expect(progress.target).toBe(50)
    expect(progress.unlocked).toBe(false)
    expect(progress.percent).toBe(50)
  })

  it('#19 已解锁成就 percent=100', () => {
    const def = ACHIEVEMENT_MAP['collector_50']
    const stats = makeStats({ totalCaptures: 60 })
    const progress = getAchievementProgress(def, stats, true)
    expect(progress.unlocked).toBe(true)
    expect(progress.percent).toBe(100)
  })
})

describe('getAllAchievementProgress', () => {
  it('#20 返回所有成就的进度', () => {
    const stats = makeStats()
    const unlockedIds = new Set<string>()
    const list = getAllAchievementProgress(stats, unlockedIds)
    expect(list.length).toBe(ACHIEVEMENT_DEFS.length)
  })
})

describe('getUnlockedTitles', () => {
  it('#21 从已解锁成就中提取称号', () => {
    const unlocked = [
      { id: 'collector_50', unlockedAt: 0 },
      { id: 'first_capture', unlockedAt: 0 },
    ]
    const titles = getUnlockedTitles(unlocked)
    expect(titles).toContain('收藏家')
    expect(titles).not.toContain('初出茅庐')
  })

  it('#22 无称号成就不返回', () => {
    const unlocked = [{ id: 'first_capture', unlockedAt: 0 }]
    const titles = getUnlockedTitles(unlocked)
    expect(titles).toEqual([])
  })
})

describe('getCompletionRate', () => {
  it('#23 0 个解锁 => 0%', () => {
    expect(getCompletionRate(0)).toBe(0)
  })

  it('#24 全部解锁 => 100%', () => {
    expect(getCompletionRate(ACHIEVEMENT_DEFS.length)).toBe(100)
  })

  it('#25 半数解锁 => 近似 50%', () => {
    const half = Math.floor(ACHIEVEMENT_DEFS.length / 2)
    expect(getCompletionRate(half)).toBe(
      Math.round((half / ACHIEVEMENT_DEFS.length) * 100)
    )
  })
})

describe('getTotalRewardGold', () => {
  it('#26 计算已解锁成就的金币总奖励', () => {
    const unlockedIds = new Set(['first_capture', 'collector_50'])
    const total = getTotalRewardGold(unlockedIds)
    expect(total).toBe(250) // 50 + 200
  })
})
