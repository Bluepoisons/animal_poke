/** PvP 排位系统 — 匹配、积分、段位 */

export type Tier =
  | 'bronze'
  | 'silver'
  | 'gold'
  | 'platinum'
  | 'diamond'
  | 'master'

export interface PlayerRating {
  playerId: string
  rating: number
  tier: Tier
  wins: number
  losses: number
  winStreak: number
}

export interface MatchResult {
  winnerId: string
  loserId: string
  ratingDelta: number
  timestamp: number
}

export const TIER_THRESHOLDS: Record<Tier, number> = {
  bronze: 0,
  silver: 1200,
  gold: 1500,
  platinum: 1800,
  diamond: 2100,
  master: 2400,
}

export const TIER_NAMES: Record<Tier, string> = {
  bronze: '青铜',
  silver: '白银',
  gold: '黄金',
  platinum: '铂金',
  diamond: '钻石',
  master: '大师',
}

/** 根据 rating 获取段位 */
export function getTierByRating(rating: number): Tier {
  if (rating >= TIER_THRESHOLDS.master) return 'master'
  if (rating >= TIER_THRESHOLDS.diamond) return 'diamond'
  if (rating >= TIER_THRESHOLDS.platinum) return 'platinum'
  if (rating >= TIER_THRESHOLDS.gold) return 'gold'
  if (rating >= TIER_THRESHOLDS.silver) return 'silver'
  return 'bronze'
}

/** ELO 积分变化计算 */
export function calculateRatingDelta(
  winnerRating: number,
  loserRating: number,
  k: number = 32,
): number {
  const expectedWinner = 1 / (1 + Math.pow(10, (loserRating - winnerRating) / 400))
  const delta = Math.round(k * (1 - expectedWinner))
  return Math.max(1, delta) // 赢家至少 +1
}

/** 应用比赛结果到玩家积分 */
export function applyMatchResult(
  winner: PlayerRating,
  loser: PlayerRating,
): { winner: PlayerRating; loser: PlayerRating; delta: number } {
  const delta = calculateRatingDelta(winner.rating, loser.rating)
  const newWinner: PlayerRating = {
    ...winner,
    rating: winner.rating + delta,
    tier: getTierByRating(winner.rating + delta),
    wins: winner.wins + 1,
    winStreak: winner.winStreak + 1,
  }
  const newLoser: PlayerRating = {
    ...loser,
    rating: Math.max(0, loser.rating - delta),
    tier: getTierByRating(Math.max(0, loser.rating - delta)),
    losses: loser.losses + 1,
    winStreak: 0,
  }
  return { winner: newWinner, loser: newLoser, delta }
}

/** 匹配搜索范围（±rating） */
export function getMatchRange(rating: number): { min: number; max: number } {
  return {
    min: Math.max(0, rating - 200),
    max: rating + 200,
  }
}

/** 检查目标是否在匹配范围内 */
export function isInMatchRange(myRating: number, targetRating: number): boolean {
  const range = getMatchRange(myRating)
  return targetRating >= range.min && targetRating <= range.max
}

/** 胜率 */
export function getWinRate(player: Pick<PlayerRating, 'wins' | 'losses'>): number {
  const total = player.wins + player.losses
  if (total === 0) return 0
  return Math.round((player.wins / total) * 100) / 100
}
