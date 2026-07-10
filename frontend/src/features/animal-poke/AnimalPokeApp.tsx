import { useState, useCallback, useRef, useEffect } from 'react'
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
  const initialScreen = ((): ScreenId => {
  const h = (typeof location !== 'undefined' ? location.hash.replace('#', '') : '') as ScreenId
  const allowed: ScreenId[] = ['discover', 'map', 'capture', 'pokedex', 'battle', 'store']
  return allowed.includes(h) ? h : 'discover'
})()
  const [screen, setScreen] = useState<ScreenId>(initialScreen)
  const [selectedTargetId, setSelectedTargetId] = useState('target-uncommon-50')
  const { state: staminaState, addGold } = useStamina()
  const currentStamina = staminaState.currentStamina
  const gold = staminaState.gold
  const [toastMessage, setToastMessage] = useState<string | null>(null)
  const toastTimer = useRef<number | null>(null)

  const showToast = useCallback((message: string) => {
    setToastMessage(message)
    if (toastTimer.current) window.clearTimeout(toastTimer.current)
    toastTimer.current = window.setTimeout(() => setToastMessage(null), 1500)
  }, [])

  const handleStartCapture = useCallback(() => setScreen('capture'), [])
  const handleNavigate = useCallback((nextScreen: ScreenId) => {
    setScreen(nextScreen)
    if (typeof history !== 'undefined') history.replaceState(null, '', `#${nextScreen}`)
  }, [])

  useEffect(() => {
    const onHash = () => {
      const h = location.hash.replace('#', '') as ScreenId
      const allowed: ScreenId[] = ['discover', 'map', 'capture', 'pokedex', 'battle', 'store']
      if ((allowed as string[]).includes(h)) setScreen(h)
    }
    window.addEventListener('hashchange', onHash)
    return () => window.removeEventListener('hashchange', onHash)
  }, [])
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
          <BottomTabBar active={screen} onChange={handleNavigate} onAchievement={handleAchievement} />
        )}
        <div className={`ap-toast ${toastMessage ? 'is-visible' : ''}`} role="status" aria-live="polite">
          {toastMessage}
        </div>
      </PhoneFrame>
    </div>
  )
}
