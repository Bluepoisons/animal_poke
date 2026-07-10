import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useCamera } from './useCamera'

describe('useCamera', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('marks unavailable when getUserMedia missing', async () => {
    // @ts-expect-error override
    navigator.mediaDevices = undefined
    const { result } = renderHook(() => useCamera())
    await act(async () => {
      await result.current.start()
    })
    expect(result.current.status).toBe('unavailable')
  })

  it('becomes ready when stream granted', async () => {
    const track = { stop: vi.fn(), enabled: true }
    const stream = { getTracks: () => [track] } as unknown as MediaStream
    // @ts-expect-error mock
    navigator.mediaDevices = {
      getUserMedia: vi.fn().mockResolvedValue(stream),
    }
    const { result } = renderHook(() => useCamera())
    await act(async () => {
      await result.current.start()
    })
    expect(result.current.status).toBe('ready')
    act(() => result.current.stop())
    expect(track.stop).toHaveBeenCalled()
  })
})
