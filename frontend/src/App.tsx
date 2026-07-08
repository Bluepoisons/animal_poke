import React, { useState, useCallback } from 'react'
import type { MainTab, CardEntry } from './types'
import { StaminaProvider } from './stamina/StaminaContext'
import TopBar from './components/TopBar'
import TabBar from './components/TabBar'
import CollectScreen from './components/CollectScreen'
import MapScreen from './components/MapScreen'
import DiscoverScreen from './components/DiscoverScreen'
import CaptureScreen from './components/CaptureScreen'
import PlaceholderScreen from './components/PlaceholderScreen'

const App: React.FC = () => {
  const [activeTab, setActiveTab] = useState<MainTab>('camera')
  const [mapOpen, setMapOpen] = useState(false)
  const [mapEntries, setMapEntries] = useState<CardEntry[]>([])
  const [mapFocus, setMapFocus] = useState<CardEntry | undefined>()

  const handleMapOpen = useCallback((entries: CardEntry[], focus?: CardEntry) => {
    setMapEntries(entries)
    setMapFocus(focus)
    setMapOpen(true)
  }, [])

  const handleMapClose = useCallback(() => {
    setMapOpen(false)
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
        return <DiscoverScreen />
      case 'fight':
        return <CaptureScreen />
      case 'store':
        return <PlaceholderScreen icon="🏪" title="Store" subtitle="道具商店 · 开发中" />
    }
  }

  return (
    <StaminaProvider>
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
    </StaminaProvider>
  )
}

export default App
