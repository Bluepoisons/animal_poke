import React, { useRef, useState, useEffect, useCallback } from 'react'

type CameraState = 'loading' | 'denied' | 'ready' | 'captured'

const DiscoverScreen: React.FC = () => {
  const videoRef = useRef<HTMLVideoElement>(null)
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const streamRef = useRef<MediaStream | null>(null)

  const [state, setState] = useState<CameraState>('loading')
  const [photoData, setPhotoData] = useState<string | null>(null)
  const [errorMsg, setErrorMsg] = useState('')

  // 打开摄像头
  const startCamera = useCallback(async () => {
    try {
      setState('loading')
      setErrorMsg('')
      const stream = await navigator.mediaDevices.getUserMedia({
        video: {
          facingMode: 'environment',
          width: { ideal: 640 },
          height: { ideal: 480 },
        },
        audio: false,
      })
      streamRef.current = stream
      if (videoRef.current) {
        videoRef.current.srcObject = stream
        await videoRef.current.play()
      }
      setState('ready')
    } catch (err: any) {
      console.warn('摄像头不可用:', err.message)
      setErrorMsg(err.message || '无法访问摄像头')
      setState('denied')
    }
  }, [])

  // 停止摄像头
  const stopCamera = useCallback(() => {
    if (streamRef.current) {
      streamRef.current.getTracks().forEach(t => t.stop())
      streamRef.current = null
    }
  }, [])

  // 拍照
  const capturePhoto = useCallback(() => {
    const video = videoRef.current
    const canvas = canvasRef.current
    if (!video || !canvas) return

    const vw = video.videoWidth
    const vh = video.videoHeight
    canvas.width = vw
    canvas.height = vh
    const ctx = canvas.getContext('2d')
    if (ctx) {
      ctx.drawImage(video, 0, 0, vw, vh)
      setPhotoData(canvas.toDataURL('image/jpeg', 0.9))
      setState('captured')
      stopCamera()
    }
  }, [stopCamera])

  // 重拍
  const retake = useCallback(() => {
    setPhotoData(null)
    startCamera()
  }, [startCamera])

  // 确认拍照（TODO: 上传到后端）
  const confirmPhoto = useCallback(() => {
    setPhotoData(null)
    startCamera()
  }, [startCamera])

  // 生命周期：打开时申请摄像头，离开组件时关闭
  useEffect(() => {
    startCamera()
    return () => stopCamera()
  }, [startCamera, stopCamera])

  // ---- 状态：加载中 ----
  if (state === 'loading') {
    return (
      <div style={styles.fullCenter}>
        <div style={styles.loadingCard}>
          <div style={styles.loadingSpinner}>📷</div>
          <h3 style={{ color: 'var(--orange-dark)', margin: '12px 0 4px' }}>正在启动摄像头</h3>
          <p style={{ color: 'var(--ink-3)', fontSize: 13 }}>请允许相机权限…</p>
        </div>
      </div>
    )
  }

  // ---- 状态：权限拒绝 ----
  if (state === 'denied') {
    return (
      <div style={styles.fullCenter}>
        <div style={styles.loadingCard}>
          <span style={{ fontSize: 48 }}>🚫</span>
          <h3 style={{ color: 'var(--orange-dark)', margin: '12px 0 4px' }}>无法访问摄像头</h3>
          <p style={{ color: 'var(--ink-3)', fontSize: 13, textAlign: 'center' }}>
            {errorMsg || '请检查浏览器权限设置'}
          </p>
          <button
            className="btn btn-primary"
            style={{ marginTop: 16, padding: '8px 20px', fontSize: 13 }}
            onClick={startCamera}
          >
            🔄 重试
          </button>
        </div>
      </div>
    )
  }

  // ---- 状态：已拍照 ----
  if (state === 'captured' && photoData) {
    return (
      <div style={styles.viewfinder}>
        <img src={photoData} alt="拍摄照片" style={styles.capturedImage} />
        {/* 检测框模拟 */}
        <div style={styles.detectionBox}>
          <span style={styles.detectionLabel}>检测完成 · 置信度 0.97</span>
        </div>
        {/* Weather */}
        <div style={styles.weatherStrip}>
          <span style={{ fontSize: 18 }}>☀️</span>
          <div>
            <div style={{ fontSize: 13, fontWeight: 600 }}>晴天 · 捕获 +5%</div>
            <div style={{ fontSize: 10, color: 'var(--ink-3)' }}>宁波·晴 宠物状态愉悦</div>
          </div>
        </div>
        {/* Action buttons */}
        <div style={styles.captureActions}>
          <button
            className="btn"
            style={styles.retakeBtn}
            onClick={retake}
          >
            📷 重拍
          </button>
          <button
            className="btn btn-primary"
            style={styles.confirmBtn}
            onClick={confirmPhoto}
          >
            🐾 开始捕获
          </button>
        </div>
      </div>
    )
  }

  // ---- 状态：就绪（实时取景） ----
  return (
    <div style={styles.viewfinder}>
      {/* 视频流 */}
      <video
        ref={videoRef}
        style={styles.video}
        playsInline
        muted
        autoPlay
      />

      {/* 隐藏画布（拍照用） */}
      <canvas ref={canvasRef} style={{ display: 'none' }} />

      {/* 顶部状态提示 */}
      <div style={styles.hint}>● 取景中 · 对准小动物</div>

      {/* 检测框 */}
      <div style={styles.detectionBox}>
        <span style={styles.detectionLabel}>寻找目标…</span>
      </div>

      {/* 裁剪线装饰 */}
      <div style={styles.crosshair}>
        <div style={styles.crosshairV} />
        <div style={styles.crosshairH} />
      </div>

      {/* 天气提示 */}
      <div style={styles.weatherStrip}>
        <span style={{ fontSize: 18 }}>☀️</span>
        <div>
          <div style={{ fontSize: 13, fontWeight: 600 }}>晴天 · 捕获 +5%</div>
          <div style={{ fontSize: 10, color: 'var(--ink-3)' }}>宁波·晴 宠物状态愉悦</div>
        </div>
      </div>

      {/* 消耗提示 */}
      <div style={styles.costNote}>
        消耗 1 食物 · 20 体力
      </div>

      {/* 拍照按钮 */}
      <button
        className="btn btn-primary"
        style={styles.captureBtn}
        onClick={capturePhoto}
      >
        ◎
      </button>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  // 全屏居中
  fullCenter: {
    flex: 1,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    background: 'var(--cream)',
    padding: 20,
  },
  loadingCard: {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    background: 'var(--white)',
    borderRadius: 20,
    padding: '32px 24px',
    boxShadow: '0 6px 0 rgba(230,115,0,0.12), 0 2px 8px rgba(74,44,26,0.08)',
    maxWidth: 280,
    width: '100%',
  },
  loadingSpinner: {
    fontSize: 40,
    animation: 'none',
  },

  // 取景区
  viewfinder: {
    flex: 1,
    background: '#000',
    position: 'relative',
    overflow: 'hidden',
  },

  // 视频流
  video: {
    position: 'absolute',
    inset: 0,
    width: '100%',
    height: '100%',
    objectFit: 'cover',
  },

  // 已拍摄照片
  capturedImage: {
    position: 'absolute',
    inset: 0,
    width: '100%',
    height: '100%',
    objectFit: 'cover',
  },

  // 顶部提示
  hint: {
    position: 'absolute',
    top: 12,
    left: '50%',
    transform: 'translateX(-50%)',
    color: '#fff',
    fontSize: 12,
    fontWeight: 600,
    textShadow: '0 1px 3px rgba(0,0,0,0.7)',
    zIndex: 2,
    letterSpacing: 0.5,
  },

  // 检测框
  detectionBox: {
    position: 'absolute',
    top: '26%',
    left: '20%',
    right: '20%',
    bottom: '48%',
    border: '3px solid var(--success)',
    borderRadius: 8,
    boxShadow: '0 0 0 9999px rgba(0,0,0,0.15)',
    zIndex: 1,
    pointerEvents: 'none',
    display: 'flex',
    alignItems: 'flex-start',
    justifyContent: 'center',
  },
  detectionLabel: {
    position: 'absolute',
    top: -24,
    left: -3,
    background: 'var(--success)',
    color: '#fff',
    fontSize: 10,
    fontWeight: 700,
    padding: '2px 8px',
    borderRadius: '0 6px 6px 0',
    whiteSpace: 'nowrap',
  },

  // 十字线
  crosshair: {
    position: 'absolute',
    inset: 0,
    pointerEvents: 'none',
    zIndex: 1,
  },
  crosshairV: {
    position: 'absolute',
    top: 0,
    left: '50%',
    width: 1,
    height: '100%',
    background: 'rgba(255,255,255,0.12)',
  },
  crosshairH: {
    position: 'absolute',
    top: '50%',
    left: 0,
    width: '100%',
    height: 1,
    background: 'rgba(255,255,255,0.12)',
  },

  // 天气
  weatherStrip: {
    position: 'absolute',
    left: '4%',
    right: '4%',
    bottom: '16%',
    background: 'rgba(255,255,255,0.94)',
    borderRadius: 16,
    display: 'flex',
    alignItems: 'center',
    gap: 10,
    padding: '10px 14px',
    boxShadow: '0 2px 12px rgba(0,0,0,0.12)',
    zIndex: 2,
  },

  // 消耗
  costNote: {
    position: 'absolute',
    left: '50%',
    bottom: '8%',
    transform: 'translateX(-50%)',
    color: '#fff',
    fontSize: 11,
    fontWeight: 600,
    textShadow: '0 1px 2px rgba(0,0,0,0.6)',
    zIndex: 2,
  },

  // 拍照按钮
  captureBtn: {
    position: 'absolute',
    bottom: '3%',
    left: '50%',
    transform: 'translateX(-50%)',
    width: 64,
    height: 64,
    borderRadius: '50%',
    border: '4px solid var(--white)',
    background: 'var(--orange)',
    boxShadow: '0 4px 0 var(--orange-dark), 0 0 0 4px rgba(255,255,255,0.25)',
    fontSize: 24,
    zIndex: 3,
    display: 'grid',
    placeItems: 'center',
    cursor: 'pointer',
  },

  // 确认/重拍按钮
  captureActions: {
    position: 'absolute',
    bottom: '4%',
    left: '10%',
    right: '10%',
    display: 'flex',
    gap: 12,
    zIndex: 3,
  },
  retakeBtn: {
    flex: 1,
    padding: '10px 0',
    fontSize: 14,
    borderRadius: 20,
    background: 'rgba(255,255,255,0.9)',
    color: 'var(--ink-2)',
    border: '2px solid var(--orange-100)',
    boxShadow: '0 3px 0 var(--orange-100)',
    fontFamily: 'inherit',
  },
  confirmBtn: {
    flex: 2,
    padding: '10px 0',
    fontSize: 15,
    borderRadius: 20,
    boxShadow: '0 4px 0 var(--orange-dark)',
    fontFamily: 'inherit',
  },
}

export default DiscoverScreen
