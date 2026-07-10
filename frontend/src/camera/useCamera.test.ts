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

  it('stops tracks from stale generation after rapid stop', async () => {
    let resolveGum!: (s: MediaStream) => void
    const track = { stop: vi.fn(), enabled: true }
    const stream = { getTracks: () => [track] } as unknown as MediaStream
    // @ts-expect-error mock
    navigator.mediaDevices = {
      getUserMedia: vi.fn().mockImplementation(
        () =>
          new Promise<MediaStream>((resolve) => {
            resolveGum = resolve
          }),
      ),
    }
    const { result } = renderHook(() => useCamera())
    let startP: Promise<void>
    act(() => {
      startP = result.current.start()
    })
    act(() => {
      result.current.stop()
    })
    await act(async () => {
      resolveGum(stream)
      await startP!
    })
    expect(track.stop).toHaveBeenCalled()
  })

  it('maps NotAllowedError to denied', async () => {
    // @ts-expect-error mock
    navigator.mediaDevices = {
      getUserMedia: vi.fn().mockRejectedValue(Object.assign(new Error('x'), { name: 'NotAllowedError' })),
    }
    const { result } = renderHook(() => useCamera())
    await act(async () => {
      await result.current.start()
    })
    expect(result.current.status).toBe('denied')
  })
})
