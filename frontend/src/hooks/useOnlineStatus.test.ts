import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useOnlineStatus } from './useOnlineStatus'

describe('useOnlineStatus', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('returns initial online status', () => {
    vi.stubGlobal('navigator', { onLine: true })
    const { result } = renderHook(() => useOnlineStatus())
    expect(result.current).toBe(true)
    vi.unstubAllGlobals()
  })

  it('updates on offline event', () => {
    vi.stubGlobal('navigator', { onLine: true })
    const { result } = renderHook(() => useOnlineStatus())
    expect(result.current).toBe(true)

    act(() => {
      window.dispatchEvent(new Event('offline'))
    })
    expect(result.current).toBe(false)
    vi.unstubAllGlobals()
  })

  it('updates on online event', () => {
    vi.stubGlobal('navigator', { onLine: false })
    const { result } = renderHook(() => useOnlineStatus())
    expect(result.current).toBe(false)

    act(() => {
      window.dispatchEvent(new Event('online'))
    })
    expect(result.current).toBe(true)
    vi.unstubAllGlobals()
  })
})
