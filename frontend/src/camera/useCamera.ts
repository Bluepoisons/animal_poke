import { useCallback, useEffect, useRef, useState, type RefObject } from 'react'
import { isForceCameraReady } from '../e2eFlags'
import { mapMediaError, oppositeFacing } from './cameraStatus'

export type CameraStatus =
  | 'idle'
  | 'requesting'
  | 'ready'
  | 'denied'
  | 'unavailable'
  | 'busy'
  | 'stopped'
  | 'insecure'
  /** Live track ended (OS preempted, unplug, or browser released). */
  | 'ended'

export type CameraFacing = 'environment' | 'user'

export type UseCameraResult = {
  status: CameraStatus
  error?: string
  facing: CameraFacing
  videoRef: RefObject<HTMLVideoElement | null>
  start: (facing?: CameraFacing) => Promise<void>
  stop: () => void
  switchFacing: () => Promise<void>
  captureFrame: (maxEdge?: number, quality?: number) => Promise<Blob | null>
  isReady: boolean
  /** Soft restart after busy / track ended / visibility return. */
  retry: () => Promise<void>
}

function stopStream(stream: MediaStream | null) {
  stream?.getTracks().forEach((t) => {
    try {
      t.stop()
    } catch {
      /* ignore */
    }
  })
}

function isInsecureHttpContext(): boolean {
  return (
    typeof window !== 'undefined' &&
    window.isSecureContext === false &&
    typeof location !== 'undefined' &&
    location.protocol === 'http:' &&
    location.hostname !== 'localhost' &&
    location.hostname !== '127.0.0.1'
  )
}

export function useCamera(): UseCameraResult {
  const [status, setStatus] = useState<CameraStatus>('idle')
  const [error, setError] = useState<string | undefined>()
  const [facing, setFacing] = useState<CameraFacing>('environment')
  const videoRef = useRef<HTMLVideoElement | null>(null)
  const streamRef = useRef<MediaStream | null>(null)
  /** 每次 start 递增；过期 generation 的 Promise 结果必须丢弃并 stop tracks */
  const genRef = useRef(0)
  const mountedRef = useRef(true)
  const facingRef = useRef<CameraFacing>('environment')
  const statusRef = useRef<CameraStatus>('idle')

  useEffect(() => {
    facingRef.current = facing
  }, [facing])
  useEffect(() => {
    statusRef.current = status
  }, [status])

  const detachTrackEnded = useRef<(() => void) | null>(null)

  const clearTrackEnded = useCallback(() => {
    detachTrackEnded.current?.()
    detachTrackEnded.current = null
  }, [])

  const bindTrackEnded = useCallback(
    (stream: MediaStream, gen: number) => {
      clearTrackEnded()
      const onEnded = () => {
        if (gen !== genRef.current || !mountedRef.current) return
        // Another track may still be live; only fail if all video tracks ended.
        const vids =
          typeof stream.getVideoTracks === 'function' ? stream.getVideoTracks() : stream.getTracks()
        const live = vids.some((t) => (t.readyState ?? 'live') === 'live')
        if (live) return
        stopStream(stream)
        if (streamRef.current === stream) streamRef.current = null
        if (videoRef.current) videoRef.current.srcObject = null
        setStatus('ended')
        setError('track_ended')
      }
      const tracks =
        typeof stream.getVideoTracks === 'function' ? stream.getVideoTracks() : stream.getTracks()
      tracks.forEach((t) => {
        if (typeof t.addEventListener === 'function') t.addEventListener('ended', onEnded)
      })
      detachTrackEnded.current = () => {
        tracks.forEach((t) => {
          if (typeof t.removeEventListener === 'function') t.removeEventListener('ended', onEnded)
        })
      }
    },
    [clearTrackEnded],
  )

  const stop = useCallback(() => {
    genRef.current += 1 // invalidate in-flight start
    clearTrackEnded()
    stopStream(streamRef.current)
    streamRef.current = null
    if (videoRef.current) videoRef.current.srcObject = null
    if (mountedRef.current) setStatus('stopped')
  }, [clearTrackEnded])

  const start = useCallback(async (nextFacing?: CameraFacing) => {
    // E2E hard-gate: skip real media devices
    if (isForceCameraReady()) {
      if (nextFacing) {
        setFacing(nextFacing)
        facingRef.current = nextFacing
      }
      setStatus('ready')
      setError(undefined)
      return
    }
    if (isInsecureHttpContext()) {
      setStatus('insecure')
      setError('insecure_context')
      return
    }
    if (!navigator.mediaDevices?.getUserMedia) {
      setStatus('unavailable')
      setError('camera_api_unavailable')
      return
    }

    const facingMode = nextFacing || facingRef.current
    if (nextFacing) {
      setFacing(nextFacing)
      facingRef.current = nextFacing
    }

    // 先停旧流，再开新 generation — 切换镜头不遗留 track
    clearTrackEnded()
    stopStream(streamRef.current)
    streamRef.current = null
    if (videoRef.current) videoRef.current.srcObject = null

    const gen = ++genRef.current
    setStatus('requesting')
    setError(undefined)

    try {
      let stream: MediaStream
      try {
        stream = await navigator.mediaDevices.getUserMedia({
          audio: false,
          video: {
            facingMode: { ideal: facingMode },
            width: { ideal: 1280 },
            height: { ideal: 720 },
          },
        })
      } catch (first) {
        // 后置不可用时 fallback 前置
        if (facingMode === 'environment') {
          stream = await navigator.mediaDevices.getUserMedia({
            audio: false,
            video: {
              facingMode: { ideal: 'user' },
              width: { ideal: 1280 },
              height: { ideal: 720 },
            },
          })
          if (gen === genRef.current) {
            setFacing('user')
            facingRef.current = 'user'
          }
        } else {
          throw first
        }
      }

      // 过期 generation：立即释放，防止指示灯常亮
      if (gen !== genRef.current || !mountedRef.current) {
        stopStream(stream)
        return
      }

      streamRef.current = stream
      bindTrackEnded(stream, gen)
      if (videoRef.current) {
        videoRef.current.srcObject = stream
        await videoRef.current.play().catch(() => {})
      }

      if (gen !== genRef.current || !mountedRef.current) {
        clearTrackEnded()
        stopStream(stream)
        streamRef.current = null
        return
      }
      setStatus('ready')
      setError(undefined)
    } catch (e: unknown) {
      if (gen !== genRef.current) return
      const name = (e as { name?: string })?.name || ''
      const mapped = mapMediaError(name)
      setStatus(mapped.status)
      setError(mapped.error)
    }
  }, [bindTrackEnded, clearTrackEnded])

  const switchFacing = useCallback(async () => {
    const next = oppositeFacing(facingRef.current)
    await start(next)
  }, [start])

  const retry = useCallback(async () => {
    await start(facingRef.current)
  }, [start])

  const captureFrame = useCallback(
    async (maxEdge = 1280, quality = 0.85) => {
      if (isForceCameraReady()) {
        // Keep the forced test frame large enough to resemble a JPEG capture.
        const bytes = new Uint8Array(2500)
        bytes[0] = 0xff
        bytes[1] = 0xd8
        bytes[2] = 0xff
        bytes[bytes.length - 2] = 0xff
        bytes[bytes.length - 1] = 0xd9
        return new Blob([bytes], { type: 'image/jpeg' })
      }
      const video = videoRef.current
      if (!video || statusRef.current !== 'ready') return null
      const vw = video.videoWidth || 640
      const vh = video.videoHeight || 480
      const scale = Math.min(1, maxEdge / Math.max(vw, vh))
      const w = Math.max(1, Math.round(vw * scale))
      const h = Math.max(1, Math.round(vh * scale))
      const canvas = document.createElement('canvas')
      canvas.width = w
      canvas.height = h
      const ctx = canvas.getContext('2d')
      if (!ctx) return null
      ctx.drawImage(video, 0, 0, w, h)
      return new Promise<Blob | null>((resolve) => {
        canvas.toBlob((b) => resolve(b), 'image/jpeg', quality)
      })
    },
    [],
  )

  useEffect(() => {
    mountedRef.current = true
    const onVis = () => {
      const stream = streamRef.current
      if (document.hidden) {
        stream?.getTracks().forEach((t) => {
          t.enabled = false
        })
        return
      }
      // Returning to foreground
      stream?.getTracks().forEach((t) => {
        t.enabled = true
      })
      // If tracks died while backgrounded, recover
      if (statusRef.current === 'ready' || statusRef.current === 'ended') {
        const vids =
          stream && typeof stream.getVideoTracks === 'function'
            ? stream.getVideoTracks()
            : stream?.getTracks() ?? []
        const live = vids.some((t) => (t.readyState ?? 'live') === 'live')
        if (!live) {
          void start(facingRef.current)
        }
      }
    }
    document.addEventListener('visibilitychange', onVis)
    return () => {
      mountedRef.current = false
      document.removeEventListener('visibilitychange', onVis)
      stop()
    }
  }, [start, stop])

  return {
    status,
    error,
    facing,
    videoRef,
    start,
    stop,
    switchFacing,
    captureFrame,
    isReady: status === 'ready',
    retry,
  }
}
