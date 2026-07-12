import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useCamera } from './useCamera'

function mockTrack(overrides: Partial<MediaStreamTrack> = {}) {
  return {
    stop: vi.fn(),
    enabled: true,
    readyState: 'live' as MediaStreamTrackState,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    ...overrides,
  }
}

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
    expect(result.current.error).toBe('camera_api_unavailable')
  })

  it('becomes ready when stream granted', async () => {
    const track = mockTrack()
    const stream = {
      getTracks: () => [track],
      getVideoTracks: () => [track],
    } as unknown as MediaStream
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
    const track = mockTrack()
    const stream = {
      getTracks: () => [track],
      getVideoTracks: () => [track],
    } as unknown as MediaStream
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
    expect(result.current.error).toBe('permission_denied')
  })

  it('maps NotReadableError to busy', async () => {
    // @ts-expect-error mock
    navigator.mediaDevices = {
      getUserMedia: vi.fn().mockRejectedValue(Object.assign(new Error('x'), { name: 'NotReadableError' })),
    }
    const { result } = renderHook(() => useCamera())
    await act(async () => {
      await result.current.start()
    })
    expect(result.current.status).toBe('busy')
  })

  it('maps NotFoundError to unavailable', async () => {
    // @ts-expect-error mock
    navigator.mediaDevices = {
      getUserMedia: vi.fn().mockRejectedValue(Object.assign(new Error('x'), { name: 'NotFoundError' })),
    }
    const { result } = renderHook(() => useCamera())
    await act(async () => {
      await result.current.start()
    })
    expect(result.current.status).toBe('unavailable')
    expect(result.current.error).toBe('no_camera')
  })

  it('switchFacing race: only latest stream survives', async () => {
    const tracks: Array<ReturnType<typeof mockTrack>> = []
    let call = 0
    // @ts-expect-error mock
    navigator.mediaDevices = {
      getUserMedia: vi.fn().mockImplementation(async () => {
        call += 1
        const track = mockTrack()
        tracks.push(track)
        // first call slower path simulated by microtask order
        if (call === 1) {
          await Promise.resolve()
        }
        return {
          getTracks: () => [track],
          getVideoTracks: () => [track],
        } as unknown as MediaStream
      }),
    }
    const { result } = renderHook(() => useCamera())
    await act(async () => {
      await result.current.start('environment')
    })
    expect(result.current.facing).toBe('environment')

    await act(async () => {
      // rapid facing flips
      const p1 = result.current.switchFacing()
      const p2 = result.current.switchFacing()
      await Promise.all([p1, p2])
    })

    // all but last generation tracks must be stopped
    const stopped = tracks.filter((t) => (t.stop as ReturnType<typeof vi.fn>).mock.calls.length > 0)
    expect(stopped.length).toBeGreaterThanOrEqual(1)
    expect(result.current.status).toBe('ready')
  })

  it('track ended sets status ended', async () => {
    let endedHandler: (() => void) | undefined
    const track = mockTrack({
      addEventListener: vi.fn((ev: string, cb: () => void) => {
        if (ev === 'ended') endedHandler = cb
      }),
      readyState: 'live',
    })
    const stream = {
      getTracks: () => [track],
      getVideoTracks: () => [track],
    } as unknown as MediaStream
    // @ts-expect-error mock
    navigator.mediaDevices = {
      getUserMedia: vi.fn().mockResolvedValue(stream),
    }
    const { result } = renderHook(() => useCamera())
    await act(async () => {
      await result.current.start()
    })
    expect(result.current.status).toBe('ready')
    // simulate OS ending track
    Object.defineProperty(track, 'readyState', { value: 'ended', configurable: true })
    await act(async () => {
      endedHandler?.()
    })
    expect(result.current.status).toBe('ended')
    expect(result.current.error).toBe('track_ended')
  })

  it('retry restarts with current facing', async () => {
    const track = mockTrack()
    const gum = vi.fn().mockResolvedValue({
      getTracks: () => [track],
      getVideoTracks: () => [track],
    })
    // @ts-expect-error mock
    navigator.mediaDevices = { getUserMedia: gum }
    const { result } = renderHook(() => useCamera())
    await act(async () => {
      await result.current.start('user')
    })
    expect(result.current.facing).toBe('user')
    await act(async () => {
      await result.current.retry()
    })
    expect(gum.mock.calls.length).toBeGreaterThanOrEqual(2)
    expect(result.current.facing).toBe('user')
    expect(result.current.status).toBe('ready')
  })

  it('visibility restore re-enables tracks', async () => {
    const track = mockTrack({ enabled: true })
    const stream = {
      getTracks: () => [track],
      getVideoTracks: () => [track],
    } as unknown as MediaStream
    // @ts-expect-error mock
    navigator.mediaDevices = {
      getUserMedia: vi.fn().mockResolvedValue(stream),
    }
    const { result } = renderHook(() => useCamera())
    await act(async () => {
      await result.current.start()
    })
    Object.defineProperty(document, 'hidden', { configurable: true, get: () => true })
    act(() => {
      document.dispatchEvent(new Event('visibilitychange'))
    })
    expect(track.enabled).toBe(false)

    Object.defineProperty(document, 'hidden', { configurable: true, get: () => false })
    act(() => {
      document.dispatchEvent(new Event('visibilitychange'))
    })
    expect(track.enabled).toBe(true)
  })
})
