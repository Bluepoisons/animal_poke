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

/** 唯一生产 Provider 树 */
export function AppProviders({ children }: { children: ReactNode }) {
  return (
    <I18nProvider>
      <ConsentGate>
        <LbsProvider>
          <WeatherProvider>
            <StaminaProvider>
              <EconomyProvider>
                <ShopProvider>
                  <StatusProvider>
                    <BattleProvider>
                      <DispatchProvider>{children}</DispatchProvider>
                    </BattleProvider>
                  </StatusProvider>
                </ShopProvider>
              </EconomyProvider>
            </StaminaProvider>
          </WeatherProvider>
        </LbsProvider>
      </ConsentGate>
    </I18nProvider>
  )
}
