import { authedRequest } from '../auth/deviceAuth'

export async function listFriends() {
  return authedRequest<{ friends: unknown[]; source: string }>({
    path: '/api/v1/social/friends',
  })
}

export async function createShare(payload: Record<string, unknown>) {
  return authedRequest({
    method: 'POST',
    path: '/api/v1/social/share',
    body: JSON.stringify(payload),
    allowRetry: true,
    idempotencyKey: `share-${Date.now()}`,
  })
}

export async function fetchOpsSummary() {
  return authedRequest({ path: '/api/v1/ops/metrics-summary' })
}
