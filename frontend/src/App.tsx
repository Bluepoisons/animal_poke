import React, { useState, useCallback } from 'react'
import type { MainTab, CardEntry } from './types'
import { StaminaProvider } from './stamina/StaminaContext'
import { useStamina } from './stamina/useStamina'
import { ShopProvider } from './shop/ShopContext'
import { useAnimalStore } from './hooks/useAnimalStore'
import TopBar from './components/TopBar'
import TabBar from './components/TabBar'
import CollectScreen from './components/CollectScreen'
import MapScreen from './components/MapScreen'
import DiscoverScreen from './components/DiscoverScreen'
import CaptureScreen from './components/CaptureScreen'
import StoreScreen from './components/StoreScreen'
import PlaceholderScreen from './components/PlaceholderScreen'

/** App 内部组件：需在 StaminaProvider 内使用 hooks */
const AppInner: React.FC = () => {
  const [activeTab, setActiveTab] = useState<MainTab>('camera')
  const [mapOpen, setMapOpen] = useState(false)
  const [mapEntries, setMapEntries] = useState<CardEntry[]>([])
  const [mapFocus, setMapFocus] = useState<CardEntry | undefined>()

  // 待捕获照片数据（DiscoverScreen 拍摄完成后传送）
  const [pendingPhoto, setPendingPhoto] = useState<string | null>(null)

  const { addAnimal } = useAnimalStore()
  const { addCapture, addGold } = useStamina()

  const handleMapOpen = useCallback((entries: CardEntry[], focus?: CardEntry) => {
    setMapEntries(entries)
    setMapFocus(focus)
    setMapOpen(true)
  }, [])

  const handleMapClose = useCallback(() => {
    setMapOpen(false)
  }, [])

  // DiscoverScreen 确认拍照 → 切换至捕获屏
  const handlePhotoConfirm = useCallback((photoData: string) => {
    setPendingPhoto(photoData)
    setActiveTab('fight')
  }, [])

  // CaptureScreen 捕获成功 → 写入 IndexedDB + 体力结算 + 切换图鉴
  const handleCaptureSuccess = useCallback((entry: CardEntry) => {
    addAnimal(entry)
    addCapture(1)
    // 随机金币掉落 10~50
    const goldDrop = Math.floor(Math.random() * 41) + 10
    addGold(goldDrop)
    setPendingPhoto(null)
    setActiveTab('collection')
  }, [addAnimal, addCapture, addGold])

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
        return (
          <CaptureScreen
            onCaptureSuccess={handleCaptureSuccess}
            onCaptureFail={handleCaptureFail}
          />
        )
      case 'store':
        return <StoreScreen />
    }
  }

  return (
    <div className="phone-frame">
      <TopBar
        location="宁波·晴"
        weather="☀️"
      />
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
    <StaminaProvider>
      <ShopProvider>
        <AppInner />
      </ShopProvider>
    </StaminaProvider>
  )
}

export default App
