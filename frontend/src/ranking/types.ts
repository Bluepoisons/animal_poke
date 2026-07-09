/** 排行榜类型 */
export type LeaderboardType = 'total' | 'perCapita' | 'progress'

export interface LeaderboardEntry {
  rank: number
  regionId: string
  regionName: string
  totalScore: number
  captureCount: number
  avgScore: number
  progressDelta: number
}

export interface LeaderboardResult {
  type: LeaderboardType
  entries: LeaderboardEntry[]
  myRegionRank: number | null
  lastUpdated: number
  season: string
}

export type RegionLevel = 'city' | 'district'

export interface RegionInfo {
  id: string
  name: string
  level: RegionLevel
  parentId: string | null
}

/** 排行榜奖励（钻石） */
export interface RankReward {
  rank: number
  diamonds: number
}

/** 城市分区排行奖励配置 */
export const RANK_REWARDS: RankReward[] = [
  { rank: 1, diamonds: 500 },
  { rank: 2, diamonds: 300 },
  { rank: 3, diamonds: 200 },
  { rank: 4, diamonds: 100 },
  { rank: 5, diamonds: 50 },
]

/** 每日结算时间 (0:00 UTC+8) */
export const DAILY_RESET_HOUR = 0

/** 排行榜数据源（前端 mock，实际从后端 /api/v1/ranking/region 获取） */
export function generateMockLeaderboard(type: LeaderboardType): LeaderboardResult {
  const regions = [
    { id: 'cn-bj-cy', name: '北京·朝阳' },
    { id: 'cn-bj-hd', name: '北京·海淀' },
    { id: 'cn-sh-pd', name: '上海·浦东' },
    { id: 'cn-sh-cn', name: '上海·徐汇' },
    { id: 'cn-gz-th', name: '广州·天河' },
    { id: 'cn-sz-ns', name: '深圳·南山' },
    { id: 'cn-cd-jj', name: '成都·锦江' },
    { id: 'cn-hz-xh', name: '杭州·西湖' },
  ]

  const entries: LeaderboardEntry[] = regions.map((r, i) => {
    const base = Math.floor(Math.random() * 50000) + 10000
    const count = Math.floor(Math.random() * 500) + 100
    return {
      rank: 0,
      regionId: r.id,
      regionName: r.name,
      totalScore: base,
      captureCount: count,
      avgScore: Math.floor(base / count),
      progressDelta: Math.floor(Math.random() * 2000) - 1000,
    }
  })

  // Sort by type
  const sortKey: Record<LeaderboardType, (e: LeaderboardEntry) => number> = {
    total: e => e.totalScore,
    perCapita: e => e.avgScore,
    progress: e => e.progressDelta,
  }
  entries.sort((a, b) => sortKey[type](b) - sortKey[type](a))
  entries.forEach((e, i) => { e.rank = i + 1 })

  return {
    type,
    entries,
    myRegionRank: Math.floor(Math.random() * entries.length) + 1,
    lastUpdated: Date.now(),
    season: 'S1',
  }
}

/** 检查是否到结算时间 */
export function isResetTime(now: Date = new Date()): boolean {
  return now.getHours() === DAILY_RESET_HOUR && now.getMinutes() < 5
}

/** 获取当前赛季标识 */
export function getCurrentSeason(now: Date = new Date()): string {
  const year = now.getFullYear()
  const month = now.getMonth() + 1
  const seasonStartMonth = Math.floor((month - 1) / 3) * 3 + 1
  return `S${year}-${String(seasonStartMonth).padStart(2, '0')}`
}
