import { authedRequest } from '../auth/deviceAuth'
import { ApiError } from '../api/client'
import {
  isFeatureEnabled,
  isFeatureUnavailableError,
  markFeatureUnavailable,
} from '../features/animal-poke/featureFlags'

function handleFeatureError(err: unknown, feature: 'social' | 'ops'): never {
  if (isFeatureUnavailableError(err)) {
    markFeatureUnavailable(feature)
  }
  throw err
}

export async function listFriends() {
  if (!isFeatureEnabled('social')) {
    throw new ApiError(501, {
      error: 'feature unavailable',
      reason_code: 'feature_unavailable',
    })
  }
  try {
    return await authedRequest<{ friends: unknown[]; source: string }>({
      path: '/api/v1/social/friends',
    })
  } catch (err) {
    handleFeatureError(err, 'social')
  }
}

export async function createShare(payload: Record<string, unknown>) {
  if (!isFeatureEnabled('social')) {
    throw new ApiError(501, {
      error: 'feature unavailable',
      reason_code: 'feature_unavailable',
    })
  }
  try {
    return await authedRequest({
      method: 'POST',
      path: '/api/v1/social/share',
      body: JSON.stringify(payload),
      allowRetry: true,
      idempotencyKey: `share-${Date.now()}`,
    })
  } catch (err) {
    handleFeatureError(err, 'social')
  }
}

export async function fetchOpsSummary() {
  if (!isFeatureEnabled('ops')) {
    throw new ApiError(501, {
      error: 'feature unavailable',
      reason_code: 'feature_unavailable',
    })
  }
  try {
    return await authedRequest({ path: '/api/v1/ops/metrics-summary' })
  } catch (err) {
    handleFeatureError(err, 'ops')
  }
}
