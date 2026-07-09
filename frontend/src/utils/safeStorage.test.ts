import { describe, it, expect, vi, beforeEach } from 'vitest'
import { safeGetItem, safeSetItem, safeRemoveItem } from './safeStorage'

describe('safeStorage', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('returns fallback for missing key', () => {
    expect(safeGetItem('nonexistent', { a: 1 })).toEqual({ a: 1 })
  })

  it('stores and retrieves values', () => {
    safeSetItem('test-key', { name: 'test', count: 42 })
    expect(safeGetItem('test-key', null)).toEqual({ name: 'test', count: 42 })
  })

  it('handles JSON parse errors gracefully', () => {
    localStorage.setItem('bad-json', '{invalid}')
    expect(safeGetItem('bad-json', 'fallback')).toBe('fallback')
  })

  it('removes items', () => {
    safeSetItem('remove-me', 'value')
    safeRemoveItem('remove-me')
    expect(safeGetItem('remove-me', null)).toBeNull()
  })

  it('handles QuotaExceededError with recovery attempt', () => {
    let throwOnce = true
    const originalSetItem = localStorage.setItem
    localStorage.setItem = vi.fn((...args: Parameters<typeof localStorage.setItem>) => {
      if (throwOnce) {
        throwOnce = false
        throw new DOMException('Quota exceeded', 'QuotaExceededError')
      }
      // Second call succeeds (mock does nothing)
    })
    const result = safeSetItem('quota-test', 'value')
    // Recovery succeeded on retry
    expect(result).toBe(true)
    localStorage.setItem = originalSetItem
  })

  it('does not throw when localStorage is unavailable', () => {
    const originalGetItem = localStorage.getItem
    localStorage.getItem = vi.fn(() => {
      throw new Error('Security error')
    })
    expect(safeGetItem('error-key', 'default')).toBe('default')
    localStorage.getItem = originalGetItem
  })
})
