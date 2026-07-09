import React, { useState, useCallback, useMemo, lazy, Suspense } from 'react'
import type { MainTab, CardEntry, SpeciesType, RarityTier } from './types'
import { LbsProvider } from './lbs/LbsContext'
import { StaminaProvider } from './stamina/StaminaContext'
import { useStamina } from './stamina/useStamina'
import { ShopProvider } from './shop/ShopContext'
import { useShop } from './shop/useShop'
import { BattleProvider } from './battle/BattleContext'
import { WeatherProvider } from './weather/WeatherContext'
import { useWeather } from './weather/useWeather'
import { StatusProvider } from './status/StatusContext'
import { useStatus } from './status/useStatus'
import { EconomyProvider } from './economy/EconomyContext'
import { useEconomy } from './economy/useEconomy'
import { DispatchProvider } from './economy/DispatchContext'
import { AchievementProvider } from './achievement/AchievementContext'
import { useAchievement } from './achievement/useAchievement'
import type { AchievementStats } from './achievement/types'
import { useAnimalStore } from './hooks/useAnimalStore'
import TopBar from './components/TopBar'
import TabBar from './components/TabBar'
import LevelUpToast from './components/LevelUpToast'
import PlaceholderScreen from './components/PlaceholderScreen'
import LoadingScreen from './components/LoadingScreen'
import { ProviderErrorBoundary } from './errors/ProviderErrorBoundary'
import { ScreenErrorBoundary } from './errors/ScreenErrorBoundary'

// Lazy-load screen components for code splitting
const CollectScreen = lazy(() => import('./components/CollectScreen'))
const MapScreen = lazy(() => import('./components/MapScreen'))
const DiscoverScreen = lazy(() => import('./components/DiscoverScreen'))
const CaptureScreen = lazy(() => import('./components/CaptureScreen'))
const BattleScreen = lazy(() => import('./components/BattleScreen'))
const StoreScreen = lazy(() => import('./components/StoreScreen'))
const DispatchScreen = lazy(() => import('./components/DispatchScreen'))
const AchievementScreen = lazy(() => import('./components/AchievementScreen'))

import type { LevelUpResult } from './stamina/types'
import type { WeatherType } from './weather/types'

/** App 内部组件：需在 StaminaProvider 内使用 hooks */
const AppInner: React.FC = () => {
  const [activeTab, setActiveTab] = useState<MainTab>('camera')
  const [mapOpen, setMapOpen] = useState(false)
  const [mapEntries, setMapEntries] = useState<CardEntry[]>([])
  const [mapFocus, setMapFocus] = useState<CardEntry | undefined>()
  const [levelUpResult, setLevelUpResult] = useState<LevelUpResult | null>(null)

  const [pendingPhoto, setPendingPhoto] = useState<string | null>(null)
  const [pendingSpecies, setPendingSpecies] = useState<SpeciesType>('cat')

  const { addAnimal } = useAnimalStore()
  const { addCapture, addGold, state: staminaState } = useStamina()
  const economy = useEconomy()
  const statusCtx = useStatus()
  const weatherCtx = useWeather()
  const shop = useShop()
  const achievement = useAchievement()

  const handleMapOpen = useCallback((entries: CardEntry[], focus?: CardEntry) => {
    setMapEntries(entries)
    setMapFocus(focus)
    setMapOpen(true)
  }, [])

  const handleMapClose = useCallback(() => {
    setMapOpen(false)
  }, [])

  const buildStats = useCallback((): AchievementStats => {
    return {
      totalCaptures: staminaState.totalCaptures,
      totalBattlesWon: staminaState.totalBattlesWon,
      totalBattles: staminaState.totalBattles,
      currentWinStreak: staminaState.currentWinStreak,
      maxWinStreak: staminaState.maxWinStreak,
      level: staminaState.level,
      checkInStreak: shop.state.checkIn.streak,
      citiesVisited: 0,
      weatherTypesExperienced: [] as WeatherType[],
      capturesByRarity: {
        common: 0, uncommon: 0, rare: 0, epic: 0, legendary: 0,
      } as Record<RarityTier, number>,
      capturesBySpecies: {
        cat: 0, goose: 0, dog: 0,
      } as Record<SpeciesType, number>,
      hasLegendary: false,
      rainCapturesNoCold: 0,
    }
  }, [staminaState, shop.state.checkIn.streak])

  const handlePhotoConfirm = useCallback((photoData: string, species: SpeciesType) => {
    setPendingPhoto(photoData)
    setPendingSpecies(species)
    setActiveTab('fight')
  }, [])

  const handleCaptureSuccess = useCallback((entry: CardEntry) => {
    addAnimal(entry)
    const result = addCapture(1, entry.rarity)
    if (result.leveledUp) {
      setLevelUpResult(result)
    }
    const goldDrop = Math.floor(Math.random() * 41) + 10
    addGold(goldDrop)
    economy.trackEarn(goldDrop, 'capture', `捕获 ${entry.no}`)
    if (result.leveledUp) {
      economy.trackEarn(result.rewardGold, 'levelup', `升级至 Lv.${result.newLevel}`)
    }

    const coldRisk = weatherCtx.getColdRisk()
    if (coldRisk.isRisky && Math.random() < coldRisk.probability) {
      statusCtx.applyCold(entry.id, 'capture')
    }

    achievement.checkAchievements(buildStats())

    setPendingPhoto(null)
    setActiveTab('collection')
  }, [addAnimal, addCapture, addGold, economy, statusCtx, weatherCtx, achievement, buildStats])

  const handleCaptureFail = useCallback(() => {
    // 留在 fight tab，不做切换
  }, [])

  const renderContent = () => {
    if (mapOpen) {
      return (
        <ScreenErrorBoundary screenName="地图">
          <MapScreen entries={mapEntries} focusEntry={mapFocus} onBack={handleMapClose} />
        </ScreenErrorBoundary>
      )
    }
    switch (activeTab) {
      case 'profile':
        return <PlaceholderScreen icon="👤" title="Profile" subtitle="个人主页 · 开发中" />
      case 'collection':
        return (
          <ScreenErrorBoundary screenName="图鉴">
            <CollectScreen onMapOpen={handleMapOpen} />
          </ScreenErrorBoundary>
        )
      case 'camera':
        return (
          <ScreenErrorBoundary screenName="发现">
            <DiscoverScreen onConfirm={handlePhotoConfirm} />
          </ScreenErrorBoundary>
        )
      case 'fight':
        if (pendingPhoto) {
          return (
            <ScreenErrorBoundary screenName="捕获">
              <CaptureScreen
                targetSpecies={pendingSpecies}
                onCaptureSuccess={handleCaptureSuccess}
                onCaptureFail={handleCaptureFail}
              />
            </ScreenErrorBoundary>
          )
        }
        return (
          <ScreenErrorBoundary screenName="战斗">
            <BattleScreen />
          </ScreenErrorBoundary>
        )
      case 'store':
        return (
          <ScreenErrorBoundary screenName="商店">
            <StoreScreen />
          </ScreenErrorBoundary>
        )
      case 'dispatch':
        return (
          <ScreenErrorBoundary screenName="派遣">
            <DispatchScreen />
          </ScreenErrorBoundary>
        )
      case 'achievement':
        return (
          <ScreenErrorBoundary screenName="成就">
            <AchievementScreen />
          </ScreenErrorBoundary>
        )
    }
  }

  return (
    <div className="phone-frame">
      <TopBar />
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden', position: 'relative' }}>
        <Suspense fallback={<LoadingScreen />}>
          {renderContent()}
        </Suspense>
        {levelUpResult && levelUpResult.leveledUp && (
          <LevelUpToast
            result={levelUpResult}
            onConfirm={() => setLevelUpResult(null)}
          />
        )}
      </div>
      {!mapOpen && (
        <TabBar activeTab={activeTab} onTabChange={setActiveTab} />
      )}
    </div>
  )
}

const App: React.FC = () => {
  return (
    <ProviderErrorBoundary providerName="Lbs">
      <LbsProvider>
        <ProviderErrorBoundary providerName="Stamina">
          <StaminaProvider>
            <ProviderErrorBoundary providerName="Economy">
              <EconomyProvider>
                <ProviderErrorBoundary providerName="Weather">
                  <WeatherProvider>
                    <ProviderErrorBoundary providerName="Shop">
                      <ShopProvider>
                        <ProviderErrorBoundary providerName="Status">
                          <StatusProvider>
                            <ProviderErrorBoundary providerName="Dispatch">
                              <DispatchProvider>
                                <ProviderErrorBoundary providerName="Battle">
                                  <BattleProvider>
                                    <ProviderErrorBoundary providerName="Achievement">
                                      <AchievementProvider>
                                        <AppInner />
                                      </AchievementProvider>
                                    </ProviderErrorBoundary>
                                  </BattleProvider>
                                </ProviderErrorBoundary>
                              </DispatchProvider>
                            </ProviderErrorBoundary>
                          </StatusProvider>
                        </ProviderErrorBoundary>
                      </ShopProvider>
                    </ProviderErrorBoundary>
                  </WeatherProvider>
                </ProviderErrorBoundary>
              </EconomyProvider>
            </ProviderErrorBoundary>
          </StaminaProvider>
        </ProviderErrorBoundary>
      </LbsProvider>
    </ProviderErrorBoundary>
  )
}

export default React.memo(App)
