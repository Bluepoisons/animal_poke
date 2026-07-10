import type { ReactNode } from 'react'
import { ConsentGate } from '../compliance/ConsentGate'
import { I18nProvider } from '../i18n'
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

/** 拒绝授权时的只读壳：可浏览图鉴，不可发现/捕获 */
function ReadOnlyShell() {
  return (
    <div className="ap-root" style={{ minHeight: '100vh', background: '#FFF8F0', padding: 16 }}>
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
        只读模式：未授权照片/定位，无法使用发现与捕获。可浏览本地图鉴。
      </div>
      <GameProviders>
        <PokedexScreen onToast={() => {}} />
      </GameProviders>
    </div>
  )
}

/** 唯一生产 Provider 树 */
export function AppProviders({ children }: { children: ReactNode }) {
  return (
    <I18nProvider>
      <ConsentGate readonlyFallback={<ReadOnlyShell />}>
        <GameProviders>{children}</GameProviders>
      </ConsentGate>
    </I18nProvider>
  )
}
