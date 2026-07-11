import { useEffect, useMemo, useRef, useState } from 'react'
import {
  getConsent,
  grantConsentAsync,
  revokeConsentAsync,
  isConsentOutdated,
  flushPendingConsent,
  type ConsentRecord,
  type ConsentScope,
  SERVER_CONSENT_VERSION,
} from './index'
import { useFocusTrap } from '../a11y'

export type ConsentGateMode = 'full' | 'readonly'

export function ConsentGate({
  children,
  readonlyFallback,
}: {
  children: React.ReactNode
  /** 拒绝授权时渲染的只读壳（图鉴/设置） */
  readonlyFallback?: React.ReactNode
}) {
  const initial = useMemo(() => getConsent(), [])
  const [record, setRecord] = useState<ConsentRecord>(initial)
  const [busy, setBusy] = useState(false)
  const [scopes, setScopes] = useState<Record<ConsentScope, boolean>>({
    photo: true,
    location: true,
    precise_location: false,
  })
  const [error, setError] = useState<string | null>(null)
  const dialogRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    void (async () => {
      const flushed = await flushPendingConsent()
      if (flushed) setRecord(flushed)
      else setRecord(getConsent())
    })()
  }, [])

  const needsGate =
    record.status === 'pending' ||
    record.status === 'offline_pending' ||
    isConsentOutdated() ||
    (record.status === 'granted' && !record.serverSynced)

  const isDenied = record.status === 'denied'
  const isGranted =
    record.status === 'granted' &&
    record.serverSynced &&
    record.scopes.includes('photo') &&
    !isConsentOutdated()
  const dialogOpen = !isGranted && (!isDenied || !readonlyFallback)
  useFocusTrap({ containerRef: dialogRef, active: dialogOpen })

  if (isGranted) {
    return <>{children}</>
  }

  // 只读模式：拒绝后仍可浏览图鉴
  if (isDenied && readonlyFallback) {
    return <>{readonlyFallback}</>
  }

  if (isDenied && !readonlyFallback) {
    return (
      <div ref={dialogRef} className="ap-trap-container" style={styles.shell} role="dialog" aria-modal="true">
        <div style={styles.card}>
          <h1 style={styles.title}>仅可浏览</h1>
          <p style={styles.p}>
            未授权相机/定位权限，无法使用发现与捕获。你仍可通过应用入口浏览本地图鉴（若父组件提供只读壳）。
          </p>
          <button
            type="button"
            style={styles.primary}
            onClick={() => {
              setRecord({ ...getConsent(), status: 'pending' })
            }}
          >
            重新授权
          </button>
        </div>
      </div>
    )
  }

  if (!needsGate && !isGranted) {
    // offline_pending 等：允许进入但提示
    if (record.status === 'offline_pending' && readonlyFallback) {
      return <>{readonlyFallback}</>
    }
  }

  const toggle = (s: ConsentScope) =>
    setScopes((prev) => ({ ...prev, [s]: !prev[s] }))

  const onGrant = async () => {
    setBusy(true)
    setError(null)
    const selected = (Object.keys(scopes) as ConsentScope[]).filter((k) => scopes[k])
    if (!selected.includes('photo')) {
      setError('发现/捕获至少需要「照片识别」授权')
      setBusy(false)
      return
    }
    const next = await grantConsentAsync(selected)
    setRecord(next)
    if (next.status === 'offline_pending') {
      setError('网络不可用：同意已本地暂存，联网后自动同步服务端')
    }
    setBusy(false)
  }

  const onDeny = async () => {
    setBusy(true)
    const next = await revokeConsentAsync()
    setRecord(next)
    setBusy(false)
  }

  return (
    <div
      ref={dialogRef}
      className="ap-trap-container"
      role="dialog"
      aria-modal="true"
      aria-labelledby="consent-title"
      style={styles.shell}
    >
      <div style={styles.card}>
        <h1 id="consent-title" style={styles.title}>
          隐私与权限说明
        </h1>
        <p style={styles.p}>
          为识别附近动物并生成图鉴，我们需要访问相机与大致位置。照片仅用于识别并经服务端处理后即时销毁，不会出售给第三方。
        </p>
        <p style={{ ...styles.p, fontSize: 12, opacity: 0.8 }}>
          同意版本：{SERVER_CONSENT_VERSION}
          {isConsentOutdated() ? '（需重新确认）' : ''}
        </p>

        <fieldset style={{ border: 0, padding: 0, marginBottom: 12 }}>
          <legend style={{ fontWeight: 700, marginBottom: 8 }}>授权范围</legend>
          {(
            [
              ['photo', '照片识别（发现/捕获必需）'],
              ['location', '大致位置（天气与发现点）'],
              ['precise_location', '精确坐标（可选，仅保存到服务端加密字段）'],
            ] as const
          ).map(([key, label]) => (
            <label key={key} style={{ display: 'flex', gap: 8, marginBottom: 6, fontSize: 14 }}>
              <input
                type="checkbox"
                checked={scopes[key]}
                onChange={() => toggle(key)}
              />
              {label}
            </label>
          ))}
        </fieldset>

        {error && (
          <p role="alert" style={{ color: '#B45309', fontSize: 13, marginBottom: 8 }}>
            {error}
          </p>
        )}

        <button type="button" disabled={busy} onClick={() => void onGrant()} style={styles.primary}>
          {busy ? '同步中…' : '同意并继续'}
        </button>
        <button type="button" disabled={busy} onClick={() => void onDeny()} style={styles.secondary}>
          暂不授权（只读）
        </button>

        {record.status === 'offline_pending' && (
          <p role="status" style={{ marginTop: 12, fontSize: 13 }}>
            本地已记录同意，等待同步服务端（version {record.version} · scopes{' '}
            {record.scopes.join(',')}）
          </p>
        )}
      </div>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  shell: {
    minHeight: '100vh',
    display: 'grid',
    placeItems: 'center',
    padding: 24,
    background: '#FFF8F0',
    color: '#4A2C1A',
  },
  card: { maxWidth: 380 },
  title: { fontSize: 20, marginBottom: 12 },
  p: { lineHeight: 1.5, marginBottom: 16 },
  primary: {
    background: '#FF8C42',
    color: '#fff',
    border: 0,
    borderRadius: 12,
    padding: '12px 16px',
    marginRight: 8,
    marginBottom: 8,
  },
  secondary: {
    borderRadius: 12,
    padding: '12px 16px',
    border: '2px solid #FFD8B5',
    background: '#fff',
  },
}
