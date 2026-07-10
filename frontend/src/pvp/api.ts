import { authedRequest } from '../auth/deviceAuth'

export async function requestPvPMatch() {
  return authedRequest<{ match_id: string; status: string; elo: number; source: string }>({
    method: 'POST',
    path: '/api/v1/pvp/match',
    body: '{}',
    allowRetry: true,
    idempotencyKey: `pvp-match-${Date.now()}`,
  })
}

export async function reportPvPResult(body: Record<string, unknown>) {
  return authedRequest({
    method: 'POST',
    path: '/api/v1/pvp/result',
    body: JSON.stringify(body),
    allowRetry: true,
    idempotencyKey: `pvp-result-${body.match_id || Date.now()}`,
  })
}
