import { describe, it, expect } from 'vitest'
import { classifySync409, extractReasonCode } from './syncConflict'

describe('syncConflict 409 classification', () => {
  it('treats duplicate_animal as synced disposition', () => {
    expect(extractReasonCode({ reason_code: 'duplicate_animal', error: 'animal already exists' })).toBe(
      'duplicate_animal',
    )
    expect(classifySync409('duplicate_animal')).toBe('treat_synced')
    expect(classifySync409('idempotency_conflict')).toBe('treat_synced')
  })

  it('marks inference failures as permanent', () => {
    expect(extractReasonCode({ reason_code: 'inference_invalid' })).toBe('inference_invalid')
    expect(extractReasonCode({ reason_code: 'inference_consumed' })).toBe('inference_consumed')
    expect(extractReasonCode({ reason_code: 'inference_expired' })).toBe('inference_expired')
    expect(classifySync409('inference_invalid')).toBe('permanent_fail')
    expect(classifySync409('inference_consumed')).toBe('permanent_fail')
    expect(classifySync409('inference_expired')).toBe('permanent_fail')
  })

  it('falls back from error text', () => {
    expect(extractReasonCode({ error: 'animal already exists' })).toBe('duplicate_animal')
    expect(extractReasonCode(undefined, 'invalid or reused inference')).toBe('inference_invalid')
  })
})
