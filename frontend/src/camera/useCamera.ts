import { useCallback, useEffect, useRef, useState, type RefObject } from 'react'

export type CameraStatus =
  | 'idle'
  | 'requesting'
  | 'ready'
  | 'denied'
  | 'unavailable'
  | 'busy'
  | 'stopped'
  | 'insecure'

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

export function useCamera(): UseCameraResult {
  const [status, setStatus] = useState<CameraStatus>('idle')
  const [error, setError] = useState<string | undefined>()
  const [facing, setFacing] = useState<CameraFacing>('environment')
  const videoRef = useRef<HTMLVideoElement | null>(null)
  const streamRef = useRef<MediaStream | null>(null)
  /** 每次 start 递增；过期 generation 的 Promise 结果必须丢弃并 stop tracks */
  const genRef = useRef(0)
  const mountedRef = useRef(true)

  const stop = useCallback(() => {
    genRef.current += 1 // invalidate in-flight start
    stopStream(streamRef.current)
    streamRef.current = null
    if (videoRef.current) videoRef.current.srcObject = null
    if (mountedRef.current) setStatus('stopped')
  }, [])

  const start = useCallback(async (nextFacing?: CameraFacing) => {
    // 生产 HTTPS 要求；jsdom/测试环境 isSecureContext 常为 false，仅在明确 http 远程时拦截
    if (
      typeof window !== 'undefined' &&
      window.isSecureContext === false &&
      typeof location !== 'undefined' &&
      location.protocol === 'http:' &&
      location.hostname !== 'localhost' &&
      location.hostname !== '127.0.0.1'
    ) {
      setStatus('insecure')
      setError('insecure_context')
      return
    }
    if (!navigator.mediaDevices?.getUserMedia) {
      setStatus('unavailable')
      setError('camera_api_unavailable')
      return
    }

    const facingMode = nextFacing || facing
    if (nextFacing) setFacing(nextFacing)

    // 先停旧流，再开新 generation
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
          if (gen === genRef.current) setFacing('user')
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
      if (videoRef.current) {
        videoRef.current.srcObject = stream
        await videoRef.current.play().catch(() => {})
      }

      if (gen !== genRef.current || !mountedRef.current) {
        stopStream(stream)
        streamRef.current = null
        return
      }
      setStatus('ready')
    } catch (e: unknown) {
      if (gen !== genRef.current) return
      const name = (e as { name?: string })?.name || ''
      if (name === 'NotAllowedError' || name === 'PermissionDeniedError') {
        setStatus('denied')
        setError('permission_denied')
      } else if (name === 'NotReadableError' || name === 'TrackStartError') {
        setStatus('busy')
        setError('camera_busy')
      } else if (name === 'NotFoundError' || name === 'DevicesNotFoundError') {
        setStatus('unavailable')
        setError('no_camera')
      } else {
        setStatus('unavailable')
        setError(name || 'camera_error')
      }
    }
  }, [facing])

  const switchFacing = useCallback(async () => {
    const next: CameraFacing = facing === 'environment' ? 'user' : 'environment'
    await start(next)
  }, [facing, start])

  const captureFrame = useCallback(
    async (maxEdge = 1280, quality = 0.85) => {
      const video = videoRef.current
      if (!video || status !== 'ready') return null
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
    [status],
  )

  useEffect(() => {
    mountedRef.current = true
    const onVis = () => {
      const enable = !document.hidden
      streamRef.current?.getTracks().forEach((t) => {
        t.enabled = enable
      })
    }
    document.addEventListener('visibilitychange', onVis)
    return () => {
      mountedRef.current = false
      document.removeEventListener('visibilitychange', onVis)
      stop()
    }
  }, [stop])

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
  }
}
