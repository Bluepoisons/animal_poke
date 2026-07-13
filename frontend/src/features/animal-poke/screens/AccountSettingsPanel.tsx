import { useCallback, useEffect, useState } from 'react'
import PageTitle from '../components/PageTitle'
import {
  bindEmail,
  changePassword,
  fetchAccount,
  forgotPassword,
  listDevices,
  loginEmail,
  logoutAccount,
  requestEmailVerify,
  resetPassword,
  revokeDevice,
  type AccountInfo,
  type DeviceInfo,
} from '../../../auth/accountAuth'

interface AccountSettingsPanelProps {
  onToast: (message: string) => void
  onClose: () => void
}

/**
 * 生产账号页（AP-063 / AP-079）：邮箱绑定/登录、验证状态、改密、找回、设备管理。
 * Mock 仅在 unit/e2e fixture 中通过 accountAuth 显式 API 使用。
 */
export default function AccountSettingsPanel({ onToast, onClose }: AccountSettingsPanelProps) {
  const [info, setInfo] = useState<AccountInfo | null>(null)
  const [devices, setDevices] = useState<DeviceInfo[]>([])
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [resetToken, setResetToken] = useState('')
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

  const emailBinding = info?.bindings?.find((b) => b.provider === 'email')

  return (
    <div className="ap-screen ap-account-panel" data-testid="account-settings">
      <PageTitle title="账号与设备" subtitle="账号 · 绑定 / 安全" rightText="关闭" rightTone="yellow" />
      <button type="button" className="ap-action-button" onClick={onClose} style={{ marginBottom: 12 }}>
        返回
      </button>

      <section className="ap-account-card">
        <h3>当前状态</h3>
        {info?.guest !== false ? (
          <p>游客模式（仅本机设备编号）。清除数据后无法恢复，建议绑定。</p>
        ) : (
          <p>
            已绑定账号 <code>{info?.accountId?.slice(0, 8)}…</code>
            {info?.displayName ? ` · ${info.displayName}` : ''}
          </p>
        )}
        {emailBinding && (
          <p data-testid="account-email-status">
            邮箱 {emailBinding.providerSubject} · {emailBinding.verified ? '已验证' : '待验证（未验证不可找回）'}
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
              info?.guest !== false ? '邮箱绑定成功（请查收验证邮件）' : '邮箱登录成功',
            )
          }
        >
          {info?.guest !== false ? '绑定邮箱' : '邮箱重新登录'}
        </button>
        {info?.guest === false && emailBinding && !emailBinding.verified && (
          <button
            type="button"
            className="ap-action-button"
            disabled={busy}
            data-testid="account-resend-verify"
            onClick={() => run(() => requestEmailVerify(email || emailBinding.providerSubject), '若邮箱待验证将发送验证邮件')}
          >
            重发验证邮件
          </button>
        )}
      </section>

      <section className="ap-account-card">
        <h3>找回密码</h3>
        <p>未验证邮箱不能找回。无论邮箱是否存在，请求结果一致。</p>
        <button
          type="button"
          className="ap-action-button"
          disabled={busy || !email}
          data-testid="account-forgot-password"
          onClick={() => run(() => forgotPassword(email), '若账号存在且已验证将发送重置邮件')}
        >
          发送重置邮件
        </button>
        <label>
          重置令牌
          <input
            value={resetToken}
            onChange={(e) => setResetToken(e.target.value)}
            type="text"
            autoComplete="one-time-code"
            placeholder="邮件中的令牌"
            data-testid="account-reset-token"
          />
        </label>
        <label>
          新密码
          <input
            value={newPassword}
            onChange={(e) => setNewPassword(e.target.value)}
            type="password"
            autoComplete="new-password"
            placeholder="新密码 ≥8"
            data-testid="account-reset-new-password"
          />
        </label>
        <button
          type="button"
          className="ap-action-button"
          disabled={busy || !resetToken || newPassword.length < 8}
          data-testid="account-reset-submit"
          onClick={() =>
            run(async () => {
              await resetPassword(resetToken, newPassword)
              setResetToken('')
              setNewPassword('')
            }, '密码已重置，请重新登录')
          }
        >
          确认重置
        </button>
      </section>

      {info?.guest === false && (
        <>
          <section className="ap-account-card">
            <h3>修改密码</h3>
            <label>
              当前密码
              <input
                value={currentPassword}
                onChange={(e) => setCurrentPassword(e.target.value)}
                type="password"
                autoComplete="current-password"
                data-testid="account-current-password"
              />
            </label>
            <label>
              新密码
              <input
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                type="password"
                autoComplete="new-password"
                data-testid="account-new-password"
              />
            </label>
            <button
              type="button"
              className="ap-action-button"
              disabled={busy || currentPassword.length < 8 || newPassword.length < 8}
              data-testid="account-change-password"
              onClick={() =>
                run(async () => {
                  await changePassword(currentPassword, newPassword)
                  setCurrentPassword('')
                  setNewPassword('')
                }, '密码已修改，其他设备会话已失效')
              }
            >
              修改密码
            </button>
          </section>

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
