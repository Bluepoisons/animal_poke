import { useMemo, useState } from 'react'
import { getConsent, grantConsent, revokeConsent } from './index'

/** 照片/定位知情同意门禁（i18n 文案可后续替换） */
export function ConsentGate({ children }: { children: React.ReactNode }) {
  const initial = useMemo(() => getConsent(), [])
  const [status, setStatus] = useState(initial.status)

  if (status !== 'granted') {
    return (
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="consent-title"
        style={{
          minHeight: '100vh',
          display: 'grid',
          placeItems: 'center',
          padding: 24,
          background: '#FFF8F0',
          color: '#4A2C1A',
        }}
      >
        <div style={{ maxWidth: 360 }}>
          <h1 id="consent-title" style={{ fontSize: 20, marginBottom: 12 }}>
            隐私与权限说明
          </h1>
          <p style={{ lineHeight: 1.5, marginBottom: 16 }}>
            为识别附近动物并生成图鉴，我们需要访问相机与大致位置。照片仅用于识别并经服务端处理后即时销毁，不会出售给第三方。
          </p>
          <button
            type="button"
            onClick={() => {
              grantConsent()
              setStatus('granted')
            }}
            style={{
              background: '#FF8C42',
              color: '#fff',
              border: 0,
              borderRadius: 12,
              padding: '12px 16px',
              marginRight: 8,
            }}
          >
            同意并继续
          </button>
          <button
            type="button"
            onClick={() => {
              revokeConsent()
              setStatus('denied')
            }}
            style={{ borderRadius: 12, padding: '12px 16px' }}
          >
            暂不授权
          </button>
          {status === 'denied' && (
            <p role="status" style={{ marginTop: 12 }}>
              未授权将无法使用发现/捕获，仍可浏览本地图鉴。
            </p>
          )}
        </div>
      </div>
    )
  }

  return <>{children}</>
}
