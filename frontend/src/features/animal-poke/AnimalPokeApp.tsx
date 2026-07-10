import { useState, useCallback, useRef, useEffect, useReducer } from 'react'
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
import { useLbs } from '../../lbs/useLbs'
import { useWeather } from '../../weather/useWeather'
import {
  canEnterCapture,
  createInitialCaptureFlow,
  reduceCaptureFlow,
  type CaptureFlowEvent,
} from './captureFlow'

import './animalPoke.css'

const TAB_SCREENS: ScreenId[] = ['discover', 'map', 'pokedex', 'battle', 'store']

function parseHashScreen(): ScreenId {
  const h = (typeof location !== 'undefined' ? location.hash.replace('#', '') : '') as ScreenId
  const allowed: ScreenId[] = ['discover', 'map', 'capture', 'pokedex', 'battle', 'store']
  return allowed.includes(h) ? h : 'discover'
}

function newAttemptId(): string {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) return crypto.randomUUID()
  return `attempt-${Date.now()}`
}

export default function AnimalPokeApp() {
  const [screen, setScreen] = useState<ScreenId>(() => {
    const h = parseHashScreen()
    // 直接访问 #capture 无状态时回 discover
    return h === 'capture' ? 'discover' : h
  })
  const [selectedTargetId, setSelectedTargetId] = useState('target-uncommon-50')
  const { state: staminaState, addGold } = useStamina()
  const lbs = useLbs()
  const weather = useWeather()
  const currentStamina = staminaState.currentStamina
  const gold = staminaState.gold
  const [toastMessage, setToastMessage] = useState<string | null>(null)
  const toastTimer = useRef<number | null>(null)

  const [flow, dispatchFlow] = useReducer(reduceCaptureFlow, undefined, createInitialCaptureFlow)
  const flowRef = useRef(flow)
  flowRef.current = flow
  const dispatch = useCallback((event: CaptureFlowEvent) => {
    dispatchFlow(event)
  }, [])

  const showToast = useCallback((message: string) => {
    setToastMessage(message)
    if (toastTimer.current) window.clearTimeout(toastTimer.current)
    toastTimer.current = window.setTimeout(() => setToastMessage(null), 1800)
  }, [])

  const navigate = useCallback((nextScreen: ScreenId) => {
    setScreen(nextScreen)
    if (typeof history !== 'undefined') history.replaceState(null, '', `#${nextScreen}`)
  }, [])

  const handleEnterCapture = useCallback(() => {
    const f = flowRef.current
    if (!f.selectedBox || !f.detectInferenceId || !f.photoBlob) {
      showToast('识别数据不完整')
      navigate('discover')
      return
    }
    if (!canEnterCapture(f) && f.phase !== 'target_confirmed') {
      // 多目标已选中但未 CONFIRM：先确认
      dispatch({ type: 'CONFIRM_TARGET' })
    }
    const attemptId = newAttemptId()
    dispatch({ type: 'ENTER_CAPTURE', attemptId })
    navigate('capture')
  }, [dispatch, navigate, showToast])

  // 路由守卫：#capture 必须有确认目标
  useEffect(() => {
    const onHash = () => {
      const h = parseHashScreen()
      if (h === 'capture') {
        const f = flowRef.current
        if (!canEnterCapture(f) && f.phase !== 'capturing') {
          showToast('请先完成发现与识别')
          navigate('discover')
          return
        }
      }
      if ((TAB_SCREENS as string[]).includes(h) || h === 'capture') {
        setScreen(h)
      }
    }
    window.addEventListener('hashchange', onHash)
    return () => window.removeEventListener('hashchange', onHash)
  }, [navigate, showToast])

  // 切走 capture 时若未完成，不保留默认鹅会话：离开 capture 且非 capturing 完成则保持 flow
  useEffect(() => {
    if (screen !== 'capture' && flow.phase === 'capturing') {
      // 允许返回查看；不自动 reset
    }
  }, [screen, flow.phase])

  const handleInvalidCapture = useCallback(() => {
    dispatch({ type: 'RESET' })
    navigate('discover')
    showToast('捕获会话无效，已返回发现')
  }, [dispatch, navigate, showToast])

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
            flow={flow}
            dispatch={dispatch}
            onNavigate={navigate}
            onEnterCapture={handleEnterCapture}
            city={
              lbs.state.cityName ||
              (lbs.state.geoStatus === 'locating'
                ? '定位中'
                : lbs.state.geoStatus === 'denied'
                  ? '定位关闭'
                  : '未知城市')
            }
            weather={
              weather.todayMeta
                ? `${weather.todayMeta.emoji}${weather.todayMeta.name}`
                : weather.state.status === 'loading'
                  ? '天气…'
                  : weather.state.source === 'internal'
                    ? '本地天气'
                    : '—'
            }
          />
        )
      case 'map':
        return (
          <HuntMapScreen
            selectedTargetId={selectedTargetId}
            onSelectTarget={setSelectedTargetId}
            onBack={() => navigate('discover')}
          />
        )
      case 'capture': {
        if (!flow.selectedBox || !flow.detectInferenceId || !flow.captureAttemptId) {
          // 守卫：无状态不允许停留
          queueMicrotask(() => handleInvalidCapture())
          return (
            <DiscoverScreen
              energy={currentStamina}
              coins={gold}
              flow={flow}
              dispatch={dispatch}
              onNavigate={navigate}
              onEnterCapture={handleEnterCapture}
              city={lbs.state.cityName || '未知城市'}
              weather={weather.todayMeta ? `${weather.todayMeta.emoji}${weather.todayMeta.name}` : '—'}
            />
          )
        }
        return (
          <CaptureScreen
            onToast={showToast}
            species={flow.selectedBox.species}
            detection={flow.selectedBox}
            detectInferenceId={flow.detectInferenceId}
            targetId={flow.targetId}
            captureAttemptId={flow.captureAttemptId}
            onInvalidAccess={handleInvalidCapture}
            onSettled={(ok) => {
              if (ok) dispatch({ type: 'COMPLETE' })
              else dispatch({ type: 'FAIL', code: 'capture_failed', message: '捕获失败' })
            }}
          />
        )
      }
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
          <BottomTabBar active={screen === 'capture' ? 'discover' : screen} onChange={navigate} onAchievement={handleAchievement} />
        )}
        <div className={`ap-toast ${toastMessage ? 'is-visible' : ''}`} role="status" aria-live="polite">
          {toastMessage}
        </div>
      </PhoneFrame>
    </div>
  )
}
