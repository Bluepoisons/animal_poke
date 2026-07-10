import { authedRequest } from '../auth/deviceAuth'

export async function fetchDailyRanking(city: string) {
  return authedRequest<{ city: string; date: string; entries: unknown[]; source: string }>({
    path: `/api/v1/ranking/daily?city=${encodeURIComponent(city)}`,
  })
}
