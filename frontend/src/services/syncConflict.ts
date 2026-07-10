/**
 * 同步 409 reason_code 分类（#165 / AP-004）
 */

export type SyncConflictReason =
  | 'duplicate_animal'
  | 'idempotency_conflict'
  | 'inference_invalid'
  | 'inference_consumed'
  | 'inference_expired'
  | 'unknown_conflict'

/** 仅这些 409 可视为「已同步成功」 */
export const SYNC_SUCCESS_409: ReadonlySet<SyncConflictReason> = new Set([
  'duplicate_animal',
  'idempotency_conflict',
])

/** 永久失败：需用户重新生成，不可盲目重试 */
export const SYNC_PERMANENT_409: ReadonlySet<SyncConflictReason> = new Set([
  'inference_invalid',
  'inference_consumed',
  'inference_expired',
])

export function extractReasonCode(body: unknown, message?: string): SyncConflictReason {
  if (body && typeof body === 'object') {
    const o = body as Record<string, unknown>
    const code = o.reason_code ?? o.reasonCode ?? o.code
    if (typeof code === 'string') {
      const c = code as SyncConflictReason
      if (
        c === 'duplicate_animal' ||
        c === 'idempotency_conflict' ||
        c === 'inference_invalid' ||
        c === 'inference_consumed' ||
        c === 'inference_expired'
      ) {
        return c
      }
    }
    const err = typeof o.error === 'string' ? o.error.toLowerCase() : ''
    if (err.includes('already exists') || err.includes('duplicate')) return 'duplicate_animal'
    if (err.includes('already consumed') || err.includes('consumed')) return 'inference_consumed'
    if (err.includes('expired')) return 'inference_expired'
    if (err.includes('inference')) return 'inference_invalid'
  }
  const m = (message || '').toLowerCase()
  if (m.includes('already exists')) return 'duplicate_animal'
  if (m.includes('consumed')) return 'inference_consumed'
  if (m.includes('expired')) return 'inference_expired'
  if (m.includes('inference')) return 'inference_invalid'
  return 'unknown_conflict'
}

export type Sync409Disposition = 'treat_synced' | 'permanent_fail' | 'retryable'

export function classifySync409(reason: SyncConflictReason): Sync409Disposition {
  if (SYNC_SUCCESS_409.has(reason)) return 'treat_synced'
  if (SYNC_PERMANENT_409.has(reason)) return 'permanent_fail'
  return 'retryable'
}
