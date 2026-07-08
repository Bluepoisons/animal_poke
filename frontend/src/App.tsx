import React, { useState, useCallback } from 'react'
import type { MainTab, CardEntry, SpeciesType } from './types'
import { LbsProvider } from './lbs/LbsContext'
import { StaminaProvider } from './stamina/StaminaContext'
import { useStamina } from './stamina/useStamina'
import { ShopProvider } from './shop/ShopContext'
import { BattleProvider } from './battle/BattleContext'
import { WeatherProvider } from './weather/WeatherContext'
import { useWeather } from './weather/useWeather'
import { StatusProvider } from './status/StatusContext'
import { useStatus } from './status/useStatus'
import { EconomyProvider } from './economy/EconomyContext'
import { useEconomy } from './economy/useEconomy'
import { DispatchProvider } from './economy/DispatchContext'
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
import PlaceholderScreen from './components/PlaceholderScreen'

/** App 内部组件：需在 StaminaProvider 内使用 hooks */
const AppInner: React.FC = () => {
  const [activeTab, setActiveTab] = useState<MainTab>('camera')
  const [mapOpen, setMapOpen] = useState(false)
  const [mapEntries, setMapEntries] = useState<CardEntry[]>([])
  const [mapFocus, setMapFocus] = useState<CardEntry | undefined>()

  // 待捕获照片数据（DiscoverScreen 拍摄完成后传送）
  const [pendingPhoto, setPendingPhoto] = useState<string | null>(null)
  // 待捕获物种（DiscoverScreen 检测后传送）
  const [pendingSpecies, setPendingSpecies] = useState<SpeciesType>('cat')

  const { addAnimal } = useAnimalStore()
  const { addCapture, addGold } = useStamina()
  const economy = useEconomy()
  const statusCtx = useStatus()
  const weatherCtx = useWeather()

  const handleMapOpen = useCallback((entries: CardEntry[], focus?: CardEntry) => {
    setMapEntries(entries)
    setMapFocus(focus)
    setMapOpen(true)
  }, [])

  const handleMapClose = useCallback(() => {
    setMapOpen(false)
  }, [])

  // DiscoverScreen 确认拍照 → 切换至捕获屏
  const handlePhotoConfirm = useCallback((photoData: string, species: SpeciesType) => {
    setPendingPhoto(photoData)
    setPendingSpecies(species)
    setActiveTab('fight')
  }, [])

  // CaptureScreen 捕获成功 → 写入 IndexedDB + 体力结算 + 切换图鉴
  const handleCaptureSuccess = useCallback((entry: CardEntry) => {
    addAnimal(entry)
    const levelUpResult = addCapture(1)
    // 随机金币掉落 10~50
    const goldDrop = Math.floor(Math.random() * 41) + 10
    addGold(goldDrop)
    // 追踪捕获产出
    economy.trackEarn(goldDrop, 'capture', `捕获 ${entry.no}`)
    // 升级奖励追踪
    if (levelUpResult.leveledUp) {
      economy.trackEarn(levelUpResult.rewardGold, 'levelup', `升级至 Lv.${levelUpResult.newLevel}`)
    }

    // 感冒判定：雨雪天捕获的宠物有概率自带感冒
    const coldRisk = weatherCtx.getColdRisk()
    if (coldRisk.isRisky && Math.random() < coldRisk.probability) {
      statusCtx.applyCold(entry.id, 'capture')
    }

    setPendingPhoto(null)
    setActiveTab('collection')
  }, [addAnimal, addCapture, addGold, economy, statusCtx, weatherCtx])

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
    }
  }

  return (
    <div className="phone-frame">
      <TopBar />
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden', position: 'relative' }}>
        {renderContent()}
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
                    <AppInner />
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
