import { authedRequest } from '../auth/deviceAuth'
import { ApiError } from '../api/client'
import {
  isFeatureEnabled,
  isFeatureUnavailableError,
  markFeatureUnavailable,
} from '../features/animal-poke/featureFlags'

function handleFeatureError(err: unknown, feature: 'ranking' | 'pvp' | 'social' | 'ops'): never {
  if (isFeatureUnavailableError(err)) {
    markFeatureUnavailable(feature)
  }
  throw err
}

export async function fetchDailyRanking(city: string) {
  if (!isFeatureEnabled('ranking')) {
    throw new ApiError(501, {
      error: 'feature unavailable',
      reason_code: 'feature_unavailable',
    })
  }
  try {
    return await authedRequest<{ city: string; date: string; entries: unknown[]; source: string }>({
      path: `/api/v1/ranking/daily?city=${encodeURIComponent(city)}`,
    })
  } catch (err) {
    handleFeatureError(err, 'ranking')
  }
}
