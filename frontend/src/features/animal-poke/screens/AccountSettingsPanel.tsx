import { useCallback, useEffect, useState } from 'react'
import PageTitle from '../components/PageTitle'
import {
  bindEmail,
  bindMockOAuth,
  fetchAccount,
  listDevices,
  loginEmail,
  loginMockOAuth,
  logoutAccount,
  revokeDevice,
  type AccountInfo,
  type DeviceInfo,
} from '../../../auth/accountAuth'

interface AccountSettingsPanelProps {
  onToast: (message: string) => void
  onClose: () => void
}

export default function AccountSettingsPanel({ onToast, onClose }: AccountSettingsPanelProps) {
  const [info, setInfo] = useState<AccountInfo | null>(null)
  const [devices, setDevices] = useState<DeviceInfo[]>([])
  const [mode, setMode] = useState<'mock' | 'email'>('mock')
  const [subject, setSubject] = useState('dev-user')
  const [oauthToken, setOauthToken] = useState('dev-secret-token')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)

  const refresh = useCallback(async () => {
    try {
      const a = await fetchAccount()
      setInfo(a)
      if (!a.guest) {
        const d = await listDevices()
        setDevices(d)
      } else {
        setDevices([])
      }
    } catch {
      setInfo({ guest: true })
    }
  }, [])

  useEffect(() => {
    void refresh()
  }, [refresh])

  const run = useCallback(
    async (fn: () => Promise<unknown>, okMsg: string) => {
      if (busy) return
      setBusy(true)
      try {
        await fn()
        onToast(okMsg)
        await refresh()
      } catch (e) {
        onToast(e instanceof Error ? e.message : '操作失败')
      } finally {
        setBusy(false)
      }
    },
    [busy, onToast, refresh],
  )

  return (
    <div className="ap-screen ap-account-panel" data-testid="account-settings">
      <PageTitle title="账号与设备" subtitle="ACCOUNT · 绑定 / 迁移" rightText="关闭" rightTone="yellow" />
      <button type="button" className="ap-action-button" onClick={onClose} style={{ marginBottom: 12 }}>
        返回
      </button>

      <section className="ap-account-card">
        <h3>当前状态</h3>
        {info?.guest !== false ? (
          <p>游客模式（仅本机设备 ID）。清除数据后无法恢复，建议绑定。</p>
        ) : (
          <p>
            已绑定账号 <code>{info?.accountId?.slice(0, 8)}…</code>
            {info?.displayName ? ` · ${info.displayName}` : ''}
          </p>
        )}
      </section>

      <section className="ap-account-card">
        <h3>{info?.guest !== false ? '绑定账号' : '登录其他设备凭证'}</h3>
        <div className="ap-account-tabs">
          <button type="button" className={mode === 'mock' ? 'is-active' : ''} onClick={() => setMode('mock')}>
            Mock OAuth
          </button>
          <button type="button" className={mode === 'email' ? 'is-active' : ''} onClick={() => setMode('email')}>
            邮箱
          </button>
        </div>
        {mode === 'mock' ? (
          <>
            <label>
              Subject
              <input value={subject} onChange={(e) => setSubject(e.target.value)} autoComplete="username" />
            </label>
            <label>
              Token（仅哈希入库）
              <input
                value={oauthToken}
                onChange={(e) => setOauthToken(e.target.value)}
                type="password"
                autoComplete="current-password"
              />
            </label>
            <button
              type="button"
              className="ap-action-button"
              disabled={busy}
              onClick={() =>
                run(
                  () =>
                    info?.guest !== false
                      ? bindMockOAuth(subject, oauthToken)
                      : loginMockOAuth(subject, oauthToken),
                  info?.guest !== false ? '绑定成功' : '登录成功',
                )
              }
            >
              {info?.guest !== false ? '绑定 Mock OAuth' : '用 Mock 重新登录'}
            </button>
          </>
        ) : (
          <>
            <label>
              邮箱
              <input value={email} onChange={(e) => setEmail(e.target.value)} type="email" autoComplete="email" />
            </label>
            <label>
              密码（≥8）
              <input
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                type="password"
                autoComplete="current-password"
              />
            </label>
            <button
              type="button"
              className="ap-action-button"
              disabled={busy}
              onClick={() =>
                run(
                  () => (info?.guest !== false ? bindEmail(email, password) : loginEmail(email, password)),
                  info?.guest !== false ? '邮箱绑定成功' : '邮箱登录成功',
                )
              }
            >
              {info?.guest !== false ? '绑定邮箱' : '邮箱重新登录'}
            </button>
          </>
        )}
      </section>

      {info?.guest === false && (
        <>
          <section className="ap-account-card">
            <h3>已关联设备</h3>
            <ul className="ap-device-list">
              {devices.map((d) => (
                <li key={d.deviceId}>
                  <span>
                    {d.deviceLabel} · {d.status}
                    {d.current ? '（本机）' : ''}
                  </span>
                  {d.status === 'active' && !d.current && (
                    <button
                      type="button"
                      disabled={busy}
                      onClick={() => run(() => revokeDevice(d.deviceId), '设备已吊销')}
                    >
                      吊销
                    </button>
                  )}
                </li>
              ))}
            </ul>
          </section>
          <button
            type="button"
            className="ap-action-button"
            disabled={busy}
            onClick={() => run(() => logoutAccount(), '已退出登录')}
          >
            退出登录
          </button>
        </>
      )}
    </div>
  )
}
