/**
 * 隐私中心 UI（AP-068）— 嵌入设置页生产路径。
 * - scope：photo / location / precise / analytics（用途/状态/版本/保留）
 * - 导出：服务端 export + 轮询 + 剪贴板/下载（失败不成功）
 * - 删除：本机设置 / 本设备 / 账号 三档区分
 */

import { useCallback, useMemo, useState } from 'react'
import { useI18n } from '../i18n'
import {
  getConsent,
  grantConsentAsync,
  revokeConsentAsync,
  SERVER_CONSENT_VERSION,
  type ConsentRecord,
  type ConsentScope,
} from '../compliance'
import { onAnalyticsConsentRevoked, setAnalyticsConsent } from '../analytics'
import { exportSettingsJson } from '../settings/logic'
import { useSettings } from '../settings/SettingsContext'
import { exportTextPayload } from './clipboard'
import { exportWithStatus, deleteWithStatus } from './api'
import { listScopeViewModels, type PrivacyScopeId, type ScopeViewModel } from './scopes'
import { setAnalyticsPref, getAnalyticsPref } from './analyticsPrefs'
import { applyLocalClear } from './localClear'

export interface PrivacyCenterProps {
  onToast?: (msg: string) => void
}

function formatTs(ms: number | null, locale: string): string {
  if (ms == null) return '—'
  try {
    return new Date(ms).toLocaleString(locale === 'zh' ? 'zh-CN' : locale === 'ja' ? 'ja-JP' : 'en-US')
  } catch {
    return String(ms)
  }
}

function statusLabel(status: ScopeViewModel['status'], t: (k: string) => string): string {
  switch (status) {
    case 'granted':
      return t('privacy.status.granted')
    case 'denied':
      return t('privacy.status.denied')
    case 'offline_pending':
      return t('privacy.status.offline')
    case 'local_only':
      return t('privacy.status.local')
    default:
      return t('privacy.status.pending')
  }
}

export default function PrivacyCenter({ onToast }: PrivacyCenterProps) {
  const { t, locale } = useI18n()
  const { settings, deleteLocalData } = useSettings()
  const [consent, setConsent] = useState<ConsentRecord>(() => getConsent())
  const [busy, setBusy] = useState<string | null>(null)
  const [requestHint, setRequestHint] = useState<string | null>(null)
  const [accountPassword, setAccountPassword] = useState('')
  const [tick, setTick] = useState(0)

  const scopes = useMemo(() => {
    void tick
    return listScopeViewModels(consent)
  }, [consent, tick])

  const refresh = useCallback(() => {
    setConsent(getConsent())
    setTick((n) => n + 1)
  }, [])

  const toastOk = useCallback(
    (msg: string) => {
      onToast?.(msg)
    },
    [onToast],
  )

  const toastErr = useCallback(
    (msg: string) => {
      onToast?.(msg)
    },
    [onToast],
  )

  const setScopeEnabled = async (id: PrivacyScopeId, enable: boolean) => {
    if (busy) return
    setBusy(`scope:${id}`)
    try {
      if (id === 'analytics') {
        if (enable) {
          setAnalyticsPref('granted')
          setAnalyticsConsent(null)
        } else {
          setAnalyticsPref('denied')
          onAnalyticsConsentRevoked()
        }
        refresh()
        toastOk(t('privacy.scope.updated'))
        return
      }

      const current = getConsent()
      const scope = id as ConsentScope
      if (enable) {
        const nextScopes = [...new Set([...current.scopes, scope])]
        const next = await grantConsentAsync(nextScopes)
        setConsent(next)
        if (next.status === 'offline_pending') {
          toastErr(t('privacy.consent.offline'))
        } else if (!next.serverSynced) {
          toastErr(t('privacy.consent.sync_failed'))
        } else {
          toastOk(t('privacy.scope.updated'))
          // 从拒绝态重新授权 photo 后刷新，使 ConsentGate 退出只读壳
          if (scope === 'photo' && current.status === 'denied' && typeof window !== 'undefined') {
            window.setTimeout(() => window.location.reload(), 300)
          }
        }
      } else {
        const next = await revokeConsentAsync([scope])
        setConsent(next)
        if (scope === 'photo' || next.status === 'denied') {
          onAnalyticsConsentRevoked()
        }
        toastOk(t('privacy.scope.updated'))
      }
      refresh()
    } catch (e) {
      toastErr(e instanceof Error ? e.message : t('privacy.error.generic'))
    } finally {
      setBusy(null)
    }
  }

  const handleLocalExport = async () => {
    if (busy) return
    setBusy('local-export')
    setRequestHint(null)
    try {
      const json = exportSettingsJson(settings)
      const result = await exportTextPayload(json, `animal-poke-settings-${Date.now()}.json`)
      if (!result.ok) {
        toastErr(t('privacy.export.clipboard_failed'))
        return
      }
      toastOk(
        result.method === 'clipboard'
          ? t('privacy.export.local_done_clipboard')
          : t('privacy.export.local_done_download'),
      )
    } catch (e) {
      toastErr(e instanceof Error ? e.message : t('privacy.export.failed'))
    } finally {
      setBusy(null)
    }
  }

  const handleServerExport = async () => {
    if (busy) return
    setBusy('server-export')
    setRequestHint(t('privacy.export.processing'))
    try {
      const { requestId, data } = await exportWithStatus({ scope: 'device' })
      setRequestHint(`${t('privacy.request.id')}: ${requestId}`)
      const json = JSON.stringify(data, null, 2)
      const result = await exportTextPayload(json, `animal-poke-export-${requestId}.json`)
      if (!result.ok) {
        // 服务端已成功但剪贴板/下载失败 — 不得显示整体成功
        toastErr(t('privacy.export.clipboard_failed'))
        return
      }
      toastOk(
        result.method === 'clipboard'
          ? t('privacy.export.server_done_clipboard')
          : t('privacy.export.server_done_download'),
      )
    } catch (e) {
      setRequestHint(null)
      toastErr(e instanceof Error ? e.message : t('privacy.export.failed'))
    } finally {
      setBusy(null)
    }
  }

  const handleClearLocalSettings = async () => {
    if (busy) return
    if (typeof window !== 'undefined' && !window.confirm(t('privacy.delete.local_confirm'))) return
    setBusy('local-clear')
    try {
      await deleteLocalData()
      await applyLocalClear('settings')
      toastOk(t('privacy.delete.local_done'))
    } catch (e) {
      toastErr(e instanceof Error ? e.message : t('privacy.delete.failed'))
    } finally {
      setBusy(null)
    }
  }

  const handleDeviceDelete = async () => {
    if (busy) return
    if (typeof window !== 'undefined' && !window.confirm(t('privacy.delete.device_confirm'))) return
    setBusy('device-delete')
    setRequestHint(t('privacy.delete.processing'))
    try {
      const { requestId } = await deleteWithStatus({ scope: 'device' })
      setRequestHint(`${t('privacy.request.id')}: ${requestId}`)
      await applyLocalClear('device')
      refresh()
      toastOk(t('privacy.delete.device_done'))
    } catch (e) {
      setRequestHint(null)
      toastErr(e instanceof Error ? e.message : t('privacy.delete.failed'))
    } finally {
      setBusy(null)
    }
  }

  const handleAccountDelete = async () => {
    if (busy) return
    if (!accountPassword || accountPassword.length < 8) {
      toastErr(t('privacy.delete.reauth_required'))
      return
    }
    if (typeof window !== 'undefined' && !window.confirm(t('privacy.delete.account_confirm'))) return
    setBusy('account-delete')
    setRequestHint(t('privacy.delete.processing'))
    try {
      const { requestId } = await deleteWithStatus({
        scope: 'account',
        confirm: 'DELETE',
        reauthPassword: accountPassword,
      })
      setRequestHint(`${t('privacy.request.id')}: ${requestId}`)
      await applyLocalClear('account')
      setAccountPassword('')
      refresh()
      toastOk(t('privacy.delete.account_done'))
    } catch (e) {
      setRequestHint(null)
      toastErr(e instanceof Error ? e.message : t('privacy.delete.failed'))
    } finally {
      setBusy(null)
    }
  }

  return (
    <div data-testid="privacy-center" style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      <div>
        <h2 style={{ fontSize: 14, color: '#6D4C41', margin: '0 0 4px' }}>
          {t('settings.section.privacy')}
        </h2>
        <p style={{ fontSize: 12, color: '#8D6E63', margin: 0 }}>
          {t('privacy.version_line', { version: SERVER_CONSENT_VERSION })}
        </p>
        <p style={{ fontSize: 12, color: '#8D6E63', margin: '4px 0 0' }}>
          {t('privacy.last_sync')}: {formatTs(consent.updatedAt, locale)}
          {consent.serverSynced ? ` · ${t('privacy.synced')}` : ` · ${t('privacy.not_synced')}`}
        </p>
      </div>

      <section data-testid="privacy-scopes" aria-label={t('privacy.scopes_title')}>
        <h3 style={{ fontSize: 13, color: '#5D4037', margin: '0 0 8px' }}>{t('privacy.scopes_title')}</h3>
        <ul style={{ listStyle: 'none', padding: 0, margin: 0, display: 'flex', flexDirection: 'column', gap: 10 }}>
          {scopes.map((row) => (
            <li
              key={row.meta.id}
              data-testid={`privacy-scope-${row.meta.id}`}
              style={{
                border: '1px solid #E0C0A0',
                borderRadius: 12,
                padding: 10,
                background: '#FFF8F0',
              }}
            >
              <div style={{ display: 'flex', justifyContent: 'space-between', gap: 8, alignItems: 'center' }}>
                <strong style={{ fontSize: 13, color: '#4A2C1A' }}>
                  {t(`privacy.scope.${row.meta.id}.name` as 'privacy.scope.photo.name')}
                </strong>
                <label style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12 }}>
                  <input
                    type="checkbox"
                    checked={row.enabled}
                    disabled={busy != null}
                    data-testid={`privacy-scope-toggle-${row.meta.id}`}
                    onChange={(e) => void setScopeEnabled(row.meta.id, e.target.checked)}
                  />
                  {statusLabel(row.status, t)}
                </label>
              </div>
              <p style={{ fontSize: 12, color: '#5D4037', margin: '6px 0 0' }}>
                {t(row.meta.purposeKey as 'privacy.scope.photo.purpose')}
              </p>
              <p style={{ fontSize: 11, color: '#8D6E63', margin: '4px 0 0' }}>
                {t('privacy.field.version')}: {row.meta.version}
                {' · '}
                {t('privacy.field.retention')}: {t(row.meta.retentionKey as 'privacy.scope.photo.retention')}
                {row.meta.serverBacked
                  ? ` · ${row.serverSynced ? t('privacy.synced') : t('privacy.not_synced')}`
                  : ` · ${t('privacy.status.local')}`}
              </p>
            </li>
          ))}
        </ul>
        <p style={{ fontSize: 11, color: '#8D6E63', marginTop: 8 }}>
          {t('privacy.analytics_pref')}: {getAnalyticsPref()}
        </p>
      </section>

      <section data-testid="privacy-export" style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
        <h3 style={{ fontSize: 13, color: '#5D4037', margin: 0 }}>{t('privacy.export.title')}</h3>
        <p style={{ fontSize: 12, color: '#8D6E63', margin: 0 }}>{t('privacy.export.desc')}</p>
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
          <button
            type="button"
            data-testid="privacy-export-local"
            disabled={busy != null}
            onClick={() => void handleLocalExport()}
          >
            {busy === 'local-export' ? t('privacy.busy') : t('privacy.export.local')}
          </button>
          <button
            type="button"
            data-testid="privacy-export-server"
            disabled={busy != null}
            onClick={() => void handleServerExport()}
          >
            {busy === 'server-export' ? t('privacy.busy') : t('privacy.export.server')}
          </button>
        </div>
      </section>

      <section data-testid="privacy-delete" style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
        <h3 style={{ fontSize: 13, color: '#5D4037', margin: 0 }}>{t('privacy.delete.title')}</h3>
        <p style={{ fontSize: 12, color: '#8D6E63', margin: 0 }}>{t('privacy.delete.desc')}</p>

        <button
          type="button"
          data-testid="privacy-delete-local"
          disabled={busy != null}
          onClick={() => void handleClearLocalSettings()}
        >
          {busy === 'local-clear' ? t('privacy.busy') : t('privacy.delete.local')}
        </button>
        <p style={{ fontSize: 11, color: '#8D6E63', margin: 0 }}>{t('privacy.delete.local_hint')}</p>

        <button
          type="button"
          data-testid="privacy-delete-device"
          disabled={busy != null}
          style={{ color: '#C62828' }}
          onClick={() => void handleDeviceDelete()}
        >
          {busy === 'device-delete' ? t('privacy.busy') : t('privacy.delete.device')}
        </button>
        <p style={{ fontSize: 11, color: '#8D6E63', margin: 0 }}>{t('privacy.delete.device_hint')}</p>

        <label style={{ fontSize: 12, color: '#5D4037', display: 'flex', flexDirection: 'column', gap: 4 }}>
          {t('privacy.delete.account_password')}
          <input
            type="password"
            autoComplete="current-password"
            data-testid="privacy-account-password"
            value={accountPassword}
            disabled={busy != null}
            onChange={(e) => setAccountPassword(e.target.value)}
            placeholder="••••••••"
            style={{ padding: 8, borderRadius: 8, border: '1px solid #E0C0A0' }}
          />
        </label>
        <button
          type="button"
          data-testid="privacy-delete-account"
          disabled={busy != null}
          style={{ color: '#B71C1C', fontWeight: 700 }}
          onClick={() => void handleAccountDelete()}
        >
          {busy === 'account-delete' ? t('privacy.busy') : t('privacy.delete.account')}
        </button>
        <p style={{ fontSize: 11, color: '#8D6E63', margin: 0 }}>{t('privacy.delete.account_hint')}</p>
      </section>

      {requestHint && (
        <p role="status" data-testid="privacy-request-hint" style={{ fontSize: 12, color: '#5D4037' }}>
          {requestHint}
        </p>
      )}
    </div>
  )
}
