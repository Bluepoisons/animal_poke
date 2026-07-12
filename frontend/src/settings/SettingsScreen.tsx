import { useEffect, useState } from 'react'
import { useI18n } from '../i18n'
import { useSettings } from './SettingsContext'
import type { Locale } from '../i18n'
import type { UserSettings } from './types'
import { isFeatureEnabled } from '../features/animal-poke/featureFlags'
import PrivacyCenter from '../privacy/PrivacyCenter'
import {
  getConsent,
  isReadOnlyMode,
  canUseDiscover,
  grantConsent,
  SERVER_CONSENT_VERSION,
  type ConsentRecord,
} from '../compliance'
import { getOrCreateDeviceId } from '../auth/deviceAuth'
import { fetchAccount, type AccountInfo } from '../auth/accountAuth'
import { resetOnboarding } from '../features/animal-poke/onboarding'

interface SettingsScreenProps {
  onToast?: (msg: string) => void
  onBack?: () => void
  /** AP-061: open account/device management panel */
  onOpenAccount?: () => void
}

function ToggleRow({
  label,
  value,
  onChange,
  onLabel,
  offLabel,
}: {
  label: string
  value: boolean
  onChange: (v: boolean) => void
  onLabel: string
  offLabel: string
}) {
  return (
    <label
      style={{
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center',
        padding: '10px 0',
        borderBottom: '1px solid #F0E0D0',
        fontSize: 14,
        color: '#4A2C1A',
      }}
    >
      <span>{label}</span>
      <button
        type="button"
        role="switch"
        aria-checked={value}
        onClick={() => onChange(!value)}
        style={{
          minWidth: 56,
          padding: '6px 10px',
          borderRadius: 999,
          border: '1px solid #E0C0A0',
          background: value ? '#FF8A4C' : '#FFF8F0',
          color: value ? '#fff' : '#6D4C41',
          cursor: 'pointer',
        }}
      >
        {value ? onLabel : offLabel}
      </button>
    </label>
  )
}

export default function SettingsScreen({ onToast, onBack, onOpenAccount }: SettingsScreenProps) {
  const { t, locale, setLocale, supportedLocales } = useI18n()
  const { settings, update } = useSettings()
  const [consent, setConsent] = useState<ConsentRecord>(() => getConsent())
  const [account, setAccount] = useState<AccountInfo | null>(null)
  const [reauthBusy, setReauthBusy] = useState(false)
  const deviceId = getOrCreateDeviceId()
  const readOnly = isReadOnlyMode(consent)

  useEffect(() => {
    let cancelled = false
    void fetchAccount()
      .then((a) => {
        if (!cancelled) setAccount(a)
      })
      .catch(() => {
        if (!cancelled) setAccount({ guest: true })
      })
    return () => {
      cancelled = true
    }
  }, [])

  const showRanking = isFeatureEnabled('ranking')
  const showPvP = isFeatureEnabled('pvp')
  const showSocial = isFeatureEnabled('social')
  const showOps = isFeatureEnabled('ops')
  const showLabs = showRanking || showPvP || showSocial || showOps

  const setBool = (key: keyof UserSettings) => (v: boolean) => {
    update({ [key]: v })
    onToast?.(t('settings.saved'))
  }

  const handleLocale = (next: Locale) => {
    setLocale(next)
    update({ locale: next })
    onToast?.(t('settings.saved'))
  }

  const localeLabel = (l: Locale) =>
    l === 'zh' ? t('settings.chinese') : t('settings.english')

  const handleReauth = () => {
    if (reauthBusy) return
    setReauthBusy(true)
    try {
      const next = grantConsent(['photo', 'location'])
      setConsent(next)
      onToast?.(
        next.serverSynced
          ? t('settings.reauth.done')
          : t('settings.reauth.pending'),
      )
    } catch (e) {
      onToast?.(e instanceof Error ? e.message : t('settings.reauth.failed'))
    } finally {
      setReauthBusy(false)
      setConsent(getConsent())
    }
  }

  const handleReplayTutorial = () => {
    try {
      resetOnboarding()
      onToast?.(t('settings.tutorial.replay_done'))
    } catch {
      onToast?.(t('settings.tutorial.replay_failed'))
    }
  }

  const guest = account?.guest !== false
  const shortDevice = deviceId.length > 12 ? `${deviceId.slice(0, 8)}…` : deviceId

  return (
    <div className="ap-screen" style={{ padding: 16, overflow: 'auto' }} data-testid="settings-screen">
      {onBack && (
        <button type="button" onClick={onBack} style={{ marginBottom: 8 }} data-testid="settings-back">
          {t('common.back')}
        </button>
      )}
      <h1 style={{ fontSize: 20, margin: '0 0 4px', color: '#4A2C1A' }}>{t('settings.title')}</h1>
      <p style={{ fontSize: 13, color: '#8D6E63', margin: '0 0 16px' }}>{t('settings.subtitle')}</p>

      {/* AP-061: account / device / guest risk — always visible */}
      <section
        style={{
          marginBottom: 16,
          padding: 12,
          borderRadius: 12,
          border: '1px solid #E0C0A0',
          background: guest ? '#FFF3E0' : '#F1F8E9',
        }}
        data-testid="settings-account-summary"
      >
        <h2 style={{ fontSize: 14, color: '#6D4C41', margin: '0 0 8px' }}>{t('settings.section.account')}</h2>
        <p style={{ fontSize: 13, color: '#4A2C1A', margin: '0 0 6px' }} data-testid="settings-guest-status">
          {guest ? t('settings.guest.risk') : t('settings.guest.bound')}
          {!guest && account?.accountId ? (
            <span>
              {' '}
              <code>{account.accountId.slice(0, 8)}…</code>
            </span>
          ) : null}
        </p>
        <p style={{ fontSize: 12, color: '#8D6E63', margin: '0 0 10px' }} data-testid="settings-device-id">
          {t('settings.device')}: <code>{shortDevice}</code>
        </p>
        <p style={{ fontSize: 12, color: '#8D6E63', margin: '0 0 10px' }} data-testid="settings-sync-status">
          {t('settings.consent')}: {consent.status}
          {consent.serverSynced ? ` · ${t('settings.consent.synced')}` : ` · ${t('settings.consent.local')}`}
          {canUseDiscover(consent) ? ` · ${t('settings.consent.discover_ok')}` : ` · ${t('settings.consent.discover_blocked')}`}
        </p>
        {onOpenAccount ? (
          <button
            type="button"
            onClick={onOpenAccount}
            data-testid="settings-open-account"
            style={{
              padding: '8px 12px',
              borderRadius: 10,
              border: '1px solid #E0C0A0',
              background: '#FFE0C8',
              cursor: 'pointer',
              marginRight: 8,
            }}
          >
            {t('settings.open_account')}
          </button>
        ) : null}
        {readOnly ? (
          <button
            type="button"
            onClick={handleReauth}
            disabled={reauthBusy}
            data-testid="settings-reauth"
            style={{
              padding: '8px 12px',
              borderRadius: 10,
              border: '1px solid #FF8A4C',
              background: '#FF8A4C',
              color: '#fff',
              cursor: 'pointer',
            }}
          >
            {reauthBusy ? t('settings.reauth.busy') : t('settings.reauth')}
          </button>
        ) : null}
      </section>

      {readOnly && (
        <section
          style={{
            marginBottom: 16,
            padding: 12,
            borderRadius: 12,
            background: '#FFEBEE',
            border: '1px solid #FFCDD2',
          }}
          data-testid="settings-readonly-banner"
        >
          <p style={{ margin: 0, fontSize: 13, color: '#B71C1C' }}>{t('settings.readonly.hint')}</p>
        </section>
      )}

      <section style={{ marginBottom: 16 }}>
        <h2 style={{ fontSize: 14, color: '#6D4C41' }}>{t('settings.language')}</h2>
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginTop: 8 }}>
          {supportedLocales.map((l) => (
            <button
              key={l}
              type="button"
              onClick={() => handleLocale(l)}
              aria-pressed={locale === l}
              style={{
                padding: '8px 12px',
                borderRadius: 10,
                border: locale === l ? '2px solid #FF8A4C' : '1px solid #E0C0A0',
                background: locale === l ? '#FFE0C8' : '#FFF8F0',
                cursor: 'pointer',
              }}
            >
              {localeLabel(l)}
            </button>
          ))}
        </div>
      </section>

      <section style={{ marginBottom: 16 }}>
        <h2 style={{ fontSize: 14, color: '#6D4C41' }}>{t('settings.section.audio')}</h2>
        <ToggleRow
          label={t('settings.sfx')}
          value={settings.sfxEnabled}
          onChange={setBool('sfxEnabled')}
          onLabel={t('common.on')}
          offLabel={t('common.off')}
        />
        <ToggleRow
          label={t('settings.music')}
          value={settings.musicEnabled}
          onChange={setBool('musicEnabled')}
          onLabel={t('common.on')}
          offLabel={t('common.off')}
        />
      </section>

      <section style={{ marginBottom: 16 }}>
        <h2 style={{ fontSize: 14, color: '#6D4C41' }}>{t('settings.section.motion')}</h2>
        <ToggleRow
          label={t('settings.haptics')}
          value={settings.hapticsEnabled}
          onChange={setBool('hapticsEnabled')}
          onLabel={t('common.on')}
          offLabel={t('common.off')}
        />
        <ToggleRow
          label={t('settings.motion')}
          value={settings.reduceMotion}
          onChange={setBool('reduceMotion')}
          onLabel={t('common.on')}
          offLabel={t('common.off')}
        />
      </section>

      <section style={{ marginBottom: 16 }}>
        <h2 style={{ fontSize: 14, color: '#6D4C41' }}>{t('settings.section.data')}</h2>
        <ToggleRow
          label={t('settings.dataSaver')}
          value={settings.dataSaver}
          onChange={setBool('dataSaver')}
          onLabel={t('common.on')}
          offLabel={t('common.off')}
        />
        <ToggleRow
          label={t('settings.homeMode')}
          value={settings.homeMode}
          onChange={setBool('homeMode')}
          onLabel={t('common.on')}
          offLabel={t('common.off')}
        />
        <p style={{ fontSize: 12, color: '#8D6E63' }} data-testid="settings-home-mode-hint">
          {t('settings.homeMode.hint')}
        </p>
        <p style={{ fontSize: 12, color: '#8D6E63' }}>{t('settings.sync.hint')}</p>
      </section>

      <section style={{ marginBottom: 16 }}>
        <h2 style={{ fontSize: 14, color: '#6D4C41' }}>{t('settings.section.help')}</h2>
        <button
          type="button"
          onClick={handleReplayTutorial}
          data-testid="settings-replay-tutorial"
          style={{
            padding: '8px 12px',
            borderRadius: 10,
            border: '1px solid #E0C0A0',
            background: '#FFF8F0',
            cursor: 'pointer',
          }}
        >
          {t('settings.tutorial.replay')}
        </button>
        <p style={{ fontSize: 12, color: '#8D6E63', marginTop: 8 }}>{t('settings.permissions.desc')}</p>
        <p style={{ fontSize: 12, color: '#8D6E63' }}>
          {t('settings.consent.version')}: {SERVER_CONSENT_VERSION}
        </p>
      </section>

      {showLabs && (
        <section style={{ marginBottom: 16 }} data-testid="feature-labs">
          <h2 style={{ fontSize: 14, color: '#6D4C41' }}>Labs</h2>
          <p style={{ fontSize: 12, color: '#8D6E63', marginBottom: 8 }}>
            Experimental surfaces (hidden when feature flags are off or API returns feature_unavailable).
          </p>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
            {showRanking && (
              <button type="button" onClick={() => onToast?.('Ranking is experimental')} data-testid="entry-ranking">
                Ranking
              </button>
            )}
            {showPvP && (
              <button type="button" onClick={() => onToast?.('PvP is experimental')} data-testid="entry-pvp">
                PvP
              </button>
            )}
            {showSocial && (
              <button type="button" onClick={() => onToast?.('Social is experimental')} data-testid="entry-social">
                Social
              </button>
            )}
            {showOps && (
              <button type="button" onClick={() => onToast?.('Ops is internal only')} data-testid="entry-ops">
                Ops
              </button>
            )}
          </div>
        </section>
      )}

      <section style={{ marginBottom: 16 }}>
        <PrivacyCenter onToast={onToast} />
      </section>
    </div>
  )
}
