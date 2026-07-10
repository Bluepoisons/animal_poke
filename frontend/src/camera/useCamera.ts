import { useCallback, useEffect, useRef, useState, type RefObject } from 'react'

export type CameraStatus =
  | 'idle'
  | 'requesting'
  | 'ready'
  | 'denied'
  | 'unavailable'
  | 'busy'
  | 'stopped'

export type UseCameraResult = {
  status: CameraStatus
  error?: string
  videoRef: RefObject<HTMLVideoElement | null>
  start: () => Promise<void>
  stop: () => void
  captureFrame: (maxEdge?: number, quality?: number) => Promise<Blob | null>
}

export function useCamera(): UseCameraResult {
  const [status, setStatus] = useState<CameraStatus>('idle')
  const [error, setError] = useState<string | undefined>()
  const videoRef = useRef<HTMLVideoElement | null>(null)
  const streamRef = useRef<MediaStream | null>(null)

  const stop = useCallback(() => {
    streamRef.current?.getTracks().forEach((t) => t.stop())
    streamRef.current = null
    if (videoRef.current) videoRef.current.srcObject = null
    setStatus('stopped')
  }, [])

  const start = useCallback(async () => {
    if (!navigator.mediaDevices?.getUserMedia) {
      setStatus('unavailable')
      setError('camera_api_unavailable')
      return
    }
    setStatus('requesting')
    setError(undefined)
    try {
      const stream = await navigator.mediaDevices.getUserMedia({
        audio: false,
        video: {
          facingMode: { ideal: 'environment' },
          width: { ideal: 1280 },
          height: { ideal: 720 },
        },
      })
      streamRef.current = stream
      if (videoRef.current) {
        videoRef.current.srcObject = stream
        await videoRef.current.play().catch(() => {})
      }
      setStatus('ready')
    } catch (e: unknown) {
      const name = (e as { name?: string })?.name || ''
      if (name === 'NotAllowedError' || name === 'PermissionDeniedError') {
        setStatus('denied')
        setError('permission_denied')
      } else if (name === 'NotReadableError' || name === 'TrackStartError') {
        setStatus('busy')
        setError('camera_busy')
      } else {
        setStatus('unavailable')
        setError(name || 'camera_error')
      }
    }
  }, [])

  const captureFrame = useCallback(async (maxEdge = 1280, quality = 0.85) => {
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
  }, [status])

  useEffect(() => {
    const onVis = () => {
      const enable = !document.hidden
      streamRef.current?.getTracks().forEach((t) => {
        t.enabled = enable
      })
    }
    document.addEventListener('visibilitychange', onVis)
    return () => {
      document.removeEventListener('visibilitychange', onVis)
      stop()
    }
  }, [stop])

  return { status, error, videoRef, start, stop, captureFrame }
}
