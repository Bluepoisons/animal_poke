import React, { useState, useCallback, useMemo } from 'react'
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
import CollectScreen from './components/CollectScreen'
import MapScreen from './components/MapScreen'
import DiscoverScreen from './components/DiscoverScreen'
import CaptureScreen from './components/CaptureScreen'
import BattleScreen from './components/BattleScreen'
import StoreScreen from './components/StoreScreen'
import DispatchScreen from './components/DispatchScreen'
import AchievementScreen from './components/AchievementScreen'
import LevelUpToast from './components/LevelUpToast'
import PlaceholderScreen from './components/PlaceholderScreen'
import type { LevelUpResult } from './stamina/types'
import type { WeatherType } from './weather/types'

/** App 内部组件：需在 StaminaProvider 内使用 hooks */
const AppInner: React.FC = () => {
  const [activeTab, setActiveTab] = useState<MainTab>('camera')
  const [mapOpen, setMapOpen] = useState(false)
  const [mapEntries, setMapEntries] = useState<CardEntry[]>([])
  const [mapFocus, setMapFocus] = useState<CardEntry | undefined>()
  const [levelUpResult, setLevelUpResult] = useState<LevelUpResult | null>(null)

  // 待捕获照片数据（DiscoverScreen 拍摄完成后传送）
  const [pendingPhoto, setPendingPhoto] = useState<string | null>(null)
  // 待捕获物种（DiscoverScreen 检测后传送）
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

  // 构建成就统计数据
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

  // DiscoverScreen 确认拍照 → 切换至捕获屏
  const handlePhotoConfirm = useCallback((photoData: string, species: SpeciesType) => {
    setPendingPhoto(photoData)
    setPendingSpecies(species)
    setActiveTab('fight')
  }, [])

  // CaptureScreen 捕获成功 → 写入 IndexedDB + 体力结算 + 切换图鉴
  const handleCaptureSuccess = useCallback((entry: CardEntry) => {
    addAnimal(entry)
    const result = addCapture(1, entry.rarity)
    if (result.leveledUp) {
      setLevelUpResult(result)
    }
    // 随机金币掉落 10~50
    const goldDrop = Math.floor(Math.random() * 41) + 10
    addGold(goldDrop)
    // 追踪捕获产出
    economy.trackEarn(goldDrop, 'capture', `捕获 ${entry.no}`)
    // 升级奖励追踪
    if (result.leveledUp) {
      economy.trackEarn(result.rewardGold, 'levelup', `升级至 Lv.${result.newLevel}`)
    }

    // 感冒判定：雨雪天捕获的宠物有概率自带感冒
    const coldRisk = weatherCtx.getColdRisk()
    if (coldRisk.isRisky && Math.random() < coldRisk.probability) {
      statusCtx.applyCold(entry.id, 'capture')
    }

    // 检查成就
    achievement.checkAchievements(buildStats())

    setPendingPhoto(null)
    setActiveTab('collection')
  }, [addAnimal, addCapture, addGold, economy, statusCtx, weatherCtx, achievement, buildStats])

  // CaptureScreen 捕获失败 → 留在捕获屏，可重试
  const handleCaptureFail = useCallback(() => {
    // 留在 fight tab，不做切换
  }, [])

  const renderContent = () => {
    if (mapOpen) {
      return <MapScreen entries={mapEntries} focusEntry={mapFocus} onBack={handleMapClose} />
    }
    switch (activeTab) {
      case 'profile':
        return <PlaceholderScreen icon="👤" title="Profile" subtitle="个人主页 · 开发中" />
      case 'collection':
        return <CollectScreen onMapOpen={handleMapOpen} />
      case 'camera':
        return <DiscoverScreen onConfirm={handlePhotoConfirm} />
      case 'fight':
        if (pendingPhoto) {
          return (
            <CaptureScreen
              targetSpecies={pendingSpecies}
              onCaptureSuccess={handleCaptureSuccess}
              onCaptureFail={handleCaptureFail}
            />
          )
        }
        return <BattleScreen />
      case 'store':
        return <StoreScreen />
      case 'dispatch':
        return <DispatchScreen />
      case 'achievement':
        return <AchievementScreen />
    }
  }

  return (
    <div className="phone-frame">
      <TopBar />
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden', position: 'relative' }}>
        {renderContent()}
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
    <LbsProvider>
      <StaminaProvider>
        <EconomyProvider>
          <WeatherProvider>
            <ShopProvider>
              <StatusProvider>
                <DispatchProvider>
                  <BattleProvider>
                    <AchievementProvider>
                      <AppInner />
                    </AchievementProvider>
                  </BattleProvider>
                </DispatchProvider>
              </StatusProvider>
            </ShopProvider>
          </WeatherProvider>
        </EconomyProvider>
      </StaminaProvider>
    </LbsProvider>
  )
}

export default App
