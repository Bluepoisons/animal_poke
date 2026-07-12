import { useState, type ReactNode } from 'react'
import { ConsentGate } from '../compliance/ConsentGate'
import { I18nProvider } from '../i18n'
import { SettingsProvider } from '../settings'
import { LbsProvider } from '../lbs/LbsContext'
import { WeatherProvider } from '../weather/WeatherContext'
import { StaminaProvider } from '../stamina/StaminaContext'
import { EconomyProvider } from '../economy/EconomyContext'
import { ShopProvider } from '../shop/ShopContext'
import { StatusProvider } from '../status/StatusContext'
import { BattleProvider } from '../battle/BattleContext'
import { DispatchProvider } from '../economy/DispatchContext'
import { ProgressionProvider } from '../progression'
import PokedexScreen from '../features/animal-poke/screens/PokedexScreen'
import SettingsScreen from '../settings/SettingsScreen'
import AccountSettingsPanel from '../features/animal-poke/screens/AccountSettingsPanel'

function GameProviders({ children }: { children: ReactNode }) {
  return (
    <LbsProvider>
      <WeatherProvider>
        <StaminaProvider>
          <ProgressionProvider>
            <EconomyProvider>
              <ShopProvider>
                <StatusProvider>
                  <BattleProvider>
                    <DispatchProvider>{children}</DispatchProvider>
                  </BattleProvider>
                </StatusProvider>
              </ShopProvider>
            </EconomyProvider>
          </ProgressionProvider>
        </StaminaProvider>
      </WeatherProvider>
    </LbsProvider>
  )
}

/**
 * AP-061 只读壳：拒绝授权后仍可达设置、权限说明、重新授权与账号绑定。
 * 不挂载完整底栏（无发现/捕获），但设置入口 ≤1 次点击。
 */
function ReadOnlyShell() {
  const [tab, setTab] = useState<'pokedex' | 'settings' | 'account'>('pokedex')
  const [toast, setToast] = useState<string | null>(null)
  return (
    <div className="ap-root" style={{ minHeight: '100vh', background: '#FFF8F0', padding: 16 }} data-testid="readonly-shell">
      <div
        role="status"
        style={{
          maxWidth: 420,
          margin: '0 auto 12px',
          padding: 12,
          borderRadius: 12,
          background: '#FFF0E0',
          color: '#4A2C1A',
          fontSize: 13,
        }}
      >
        只读模式：未授权照片/定位，无法使用发现与捕获。可浏览本地图鉴、打开设置重新授权或绑定账号。
      </div>
      <div
        role="tablist"
        aria-label="只读模式"
        style={{
          maxWidth: 420,
          margin: '0 auto 12px',
          display: 'flex',
          gap: 8,
          flexWrap: 'wrap',
        }}
      >
        <button
          type="button"
          role="tab"
          aria-selected={tab === 'pokedex'}
          data-testid="readonly-tab-pokedex"
          onClick={() => setTab('pokedex')}
        >
          图鉴
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={tab === 'settings'}
          data-testid="readonly-tab-settings"
          onClick={() => setTab('settings')}
        >
          设置
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={tab === 'account'}
          data-testid="readonly-tab-account"
          onClick={() => setTab('account')}
        >
          账号
        </button>
      </div>
      {toast && (
        <p role="status" style={{ maxWidth: 420, margin: '0 auto 8px', fontSize: 13 }}>
          {toast}
        </p>
      )}
      <GameProviders>
        {tab === 'settings' ? (
          <SettingsScreen
            onToast={(m) => setToast(m)}
            onOpenAccount={() => setTab('account')}
          />
        ) : tab === 'account' ? (
          <AccountSettingsPanel onToast={(m) => setToast(m)} onClose={() => setTab('settings')} />
        ) : (
          <PokedexScreen onToast={(m) => setToast(m)} />
        )}
      </GameProviders>
    </div>
  )
}

/** 唯一生产 Provider 树 */
export function AppProviders({ children }: { children: ReactNode }) {
  return (
    <I18nProvider>
      <SettingsProvider>
        <ConsentGate readonlyFallback={<ReadOnlyShell />}>
          <GameProviders>{children}</GameProviders>
        </ConsentGate>
      </SettingsProvider>
    </I18nProvider>
  )
}
