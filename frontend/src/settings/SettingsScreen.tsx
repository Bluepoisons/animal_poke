import { useI18n } from '../i18n'
import { useSettings } from './SettingsContext'
import type { Locale } from '../i18n'
import type { UserSettings } from './types'
import { isFeatureEnabled } from '../features/animal-poke/featureFlags'
import PrivacyCenter from '../privacy/PrivacyCenter'

interface SettingsScreenProps {
  onToast?: (msg: string) => void
  onBack?: () => void
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

export default function SettingsScreen({ onToast, onBack }: SettingsScreenProps) {
  const { t, locale, setLocale, supportedLocales } = useI18n()
  const { settings, update } = useSettings()

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

  return (
    <div className="ap-screen" style={{ padding: 16, overflow: 'auto' }} data-testid="settings-screen">
      {onBack && (
        <button type="button" onClick={onBack} style={{ marginBottom: 8 }}>
          {t('common.back')}
        </button>
      )}
      <h1 style={{ fontSize: 20, margin: '0 0 4px', color: '#4A2C1A' }}>{t('settings.title')}</h1>
      <p style={{ fontSize: 13, color: '#8D6E63', margin: '0 0 16px' }}>{t('settings.subtitle')}</p>

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
        <p style={{ fontSize: 12, color: '#8D6E63' }}>{t('settings.sync.hint')}</p>
      </section>

      {/* AP-042: only show unfinished product surfaces when feature flags are on */}
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

      {/* AP-068 隐私中心：scope + 服务端导出/删除 */}
      <section style={{ marginBottom: 16 }}>
        <PrivacyCenter onToast={onToast} />
      </section>
    </div>
  )
}
