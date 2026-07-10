import { useState, useCallback, useRef } from 'react'
import type { ScreenId } from './data/types'
import PhoneFrame from './components/PhoneFrame'
import BottomTabBar from './components/BottomTabBar'
import DiscoverScreen from './screens/DiscoverScreen'
import HuntMapScreen from './screens/HuntMapScreen'
import CaptureScreen from './screens/CaptureScreen'
import PokedexScreen from './screens/PokedexScreen'
import BattleArenaScreen from './screens/BattleArenaScreen'
import StoreScreen from './screens/StoreScreen'
import { useStamina } from '../../stamina/useStamina'

import './animalPoke.css'

export default function AnimalPokeApp() {
  const [screen, setScreen] = useState<ScreenId>('discover')
  const [selectedTargetId, setSelectedTargetId] = useState('target-uncommon-50')
  const { currentStamina, gold, addGold } = useStamina()
  const [toastMessage, setToastMessage] = useState<string | null>(null)
  const toastTimer = useRef<number | null>(null)

  const showToast = useCallback((message: string) => {
    setToastMessage(message)
    if (toastTimer.current) window.clearTimeout(toastTimer.current)
    toastTimer.current = window.setTimeout(() => setToastMessage(null), 1500)
  }, [])

  const handleStartCapture = useCallback(() => setScreen('capture'), [])
  const handleNavigate = useCallback((nextScreen: ScreenId) => setScreen(nextScreen), [])
  const handleAchievement = useCallback(() => showToast('成就暂未开放'), [showToast])
  const handleCoinsChange = useCallback(
    (next: number) => {
      const delta = next - gold
      if (delta > 0) addGold(delta)
    },
    [gold, addGold],
  )

  const renderScreen = () => {
    switch (screen) {
      case 'discover':
        return (
          <DiscoverScreen
            energy={currentStamina}
            coins={gold}
            onStartCapture={handleStartCapture}
            onNavigate={handleNavigate}
          />
        )
      case 'map':
        return (
          <HuntMapScreen
            selectedTargetId={selectedTargetId}
            onSelectTarget={setSelectedTargetId}
            onBack={() => setScreen('discover')}
          />
        )
      case 'capture':
        return <CaptureScreen onToast={showToast} />
      case 'pokedex':
        return <PokedexScreen onToast={showToast} />
      case 'battle':
        return <BattleArenaScreen />
      case 'store':
        return <StoreScreen coins={gold} onCoinsChange={handleCoinsChange} onToast={showToast} />
      default:
        return null
    }
  }

  return (
    <div className="ap-root">
      <PhoneFrame variant={screen}>
        {renderScreen()}
        {screen !== 'map' && (
          <BottomTabBar active={screen} onChange={setScreen} onAchievement={handleAchievement} />
        )}
        <div className={`ap-toast ${toastMessage ? 'is-visible' : ''}`} role="status" aria-live="polite">
          {toastMessage}
        </div>
      </PhoneFrame>
    </div>
  )
}
