import { useCallback, useEffect, useState } from 'react'
import PageTitle from '../components/PageTitle'
import {
  bindEmail,
  fetchAccount,
  listDevices,
  loginEmail,
  logoutAccount,
  revokeDevice,
  type AccountInfo,
  type DeviceInfo,
} from '../../../auth/accountAuth'

interface AccountSettingsPanelProps {
  onToast: (message: string) => void
  onClose: () => void
}

/**
 * 生产账号页（AP-063）：仅邮箱绑定/登录，无 Mock OAuth、无预填开发凭证。
 * Mock 仅在 unit/e2e fixture 中通过 accountAuth 显式 API 使用。
 */
export default function AccountSettingsPanel({ onToast, onClose }: AccountSettingsPanelProps) {
  const [info, setInfo] = useState<AccountInfo | null>(null)
  const [devices, setDevices] = useState<DeviceInfo[]>([])
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
        <label>
          邮箱
          <input
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            type="email"
            autoComplete="email"
            placeholder="you@example.com"
            data-testid="account-email"
          />
        </label>
        <label>
          密码（≥8）
          <input
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            type="password"
            autoComplete="current-password"
            placeholder="密码"
            data-testid="account-password"
          />
        </label>
        <button
          type="button"
          className="ap-action-button"
          disabled={busy}
          data-testid="account-email-submit"
          onClick={() =>
            run(
              () => (info?.guest !== false ? bindEmail(email, password) : loginEmail(email, password)),
              info?.guest !== false ? '邮箱绑定成功' : '邮箱登录成功',
            )
          }
        >
          {info?.guest !== false ? '绑定邮箱' : '邮箱重新登录'}
        </button>
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
