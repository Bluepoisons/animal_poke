import { authedRequest } from '../auth/deviceAuth'
import { ApiError } from '../api/client'
import {
  isFeatureEnabled,
  isFeatureUnavailableError,
  markFeatureUnavailable,
} from '../features/animal-poke/featureFlags'

function handleFeatureError(err: unknown, feature: 'pvp'): never {
  if (isFeatureUnavailableError(err)) {
    markFeatureUnavailable(feature)
  }
  throw err
}

export async function requestPvPMatch() {
  if (!isFeatureEnabled('pvp')) {
    throw new ApiError(501, {
      error: 'feature unavailable',
      reason_code: 'feature_unavailable',
    })
  }
  try {
    return await authedRequest<{ match_id: string; status: string; elo: number; source: string }>({
      method: 'POST',
      path: '/api/v1/pvp/match',
      body: '{}',
      allowRetry: true,
      idempotencyKey: `pvp-match-${Date.now()}`,
    })
  } catch (err) {
    handleFeatureError(err, 'pvp')
  }
}

export async function reportPvPResult(body: Record<string, unknown>) {
  if (!isFeatureEnabled('pvp')) {
    throw new ApiError(501, {
      error: 'feature unavailable',
      reason_code: 'feature_unavailable',
    })
  }
  try {
    return await authedRequest({
      method: 'POST',
      path: '/api/v1/pvp/result',
      body: JSON.stringify(body),
      allowRetry: true,
      idempotencyKey: `pvp-result-${body.match_id || Date.now()}`,
    })
  } catch (err) {
    handleFeatureError(err, 'pvp')
  }
}
