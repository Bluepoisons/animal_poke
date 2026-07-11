import { useState, useCallback, useRef, useEffect, useReducer, useMemo } from 'react'
import type { ScreenId } from './data/types'
import PhoneFrame from './components/PhoneFrame'
import BottomTabBar from './components/BottomTabBar'
import DailyGoalsPanel from './components/DailyGoalsPanel'
import DiscoverScreen from './screens/DiscoverScreen'
import HuntMapScreen from './screens/HuntMapScreen'
import CaptureScreen from './screens/CaptureScreen'
import PokedexScreen from './screens/PokedexScreen'
import BattleArenaScreen from './screens/BattleArenaScreen'
import StoreScreen from './screens/StoreScreen'
import SettingsScreen from '../../settings/SettingsScreen'
import AccountSettingsPanel from './screens/AccountSettingsPanel'
import { useStamina } from '../../stamina/useStamina'
import { FEATURE_FLAGS } from './featureFlags'
import { useLbs } from '../../lbs/useLbs'
import { useWeather } from '../../weather/useWeather'
import { useProgression } from '../../progression'
import type { FeatureId } from '../../progression'
import {
  canEnterCapture,
  createInitialCaptureFlow,
  reduceCaptureFlow,
  type CaptureFlowEvent,
} from './captureFlow'
import { RouteAnnouncerElement } from '../../a11y'
import OnboardingOverlay from './components/OnboardingOverlay'
import { setCaptureActive } from '../../pwa/updateGate'

const TAB_SCREENS: ScreenId[] = ['discover', 'map', 'pokedex', 'battle', 'store', 'settings']

function parseHashScreen(): ScreenId {
  const h = (typeof location !== 'undefined' ? location.hash.replace('#', '') : '') as ScreenId
  const allowed: ScreenId[] = ['discover', 'map', 'capture', 'pokedex', 'battle', 'store', 'settings']
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
const progression = useProgression()
  const level = staminaState.level
  const exp = staminaState.exp
  const lbs = useLbs()
  const weather = useWeather()
  const currentStamina = staminaState.currentStamina
  const gold = staminaState.gold
  const [toastMessage, setToastMessage] = useState<string | null>(null)
  const [showAccount, setShowAccount] = useState(false)
  const toastTimer = useRef<number | null>(null)

  const [flow, dispatchFlow] = useReducer(reduceCaptureFlow, undefined, createInitialCaptureFlow)
  const flowRef = useRef(flow)
  flowRef.current = flow
  // AP-040: block SW apply mid-capture
  useEffect(() => {
    const phase = String((flow as { phase?: string }).phase || '')
    const active =
      screen === 'capture' ||
      phase === 'throwing' ||
      phase === 'generating' ||
      phase === 'settling' ||
      phase === 'analyzing'
    setCaptureActive(active)
  }, [screen, flow])
  const dispatch = useCallback((event: CaptureFlowEvent) => {
    dispatchFlow(event)
  }, [])

  const showToast = useCallback((message: string) => {
    setToastMessage(message)
    if (toastTimer.current) window.clearTimeout(toastTimer.current)
    toastTimer.current = window.setTimeout(() => setToastMessage(null), 1800)
  }, [])

  const navigate = useCallback(
    (nextScreen: ScreenId, opts?: { replace?: boolean }) => {
      // Hide locked features instead of toast spam
      const feature = nextScreen as FeatureId
      if (
        (feature === 'battle' ||
          feature === 'map' ||
          feature === 'pokedex' ||
          feature === 'store' ||
          feature === 'discover') &&
        !progression.isFeatureUnlocked(feature)
      ) {
        return
      }
      setScreen(nextScreen)
      if (typeof history === 'undefined') return
      const url = `#${nextScreen}`
      if (opts?.replace) history.replaceState({ screen: nextScreen }, '', url)
      else history.pushState({ screen: nextScreen }, '', url)
    },
    [progression],
  )

  const handleEnterCapture = useCallback(() => {
    const f = flowRef.current
    if (!f.selectedBox || !f.detectInferenceId || !f.photoBlob) {
      showToast('识别数据不完整')
      navigate('discover', { replace: true })
      return
    }
    if (!canEnterCapture(f) && f.phase !== 'target_confirmed') {
      dispatch({ type: 'CONFIRM_TARGET' })
    }
    const attemptId = newAttemptId()
    dispatch({ type: 'ENTER_CAPTURE', attemptId })
    navigate('capture')
  }, [dispatch, navigate, showToast])

  // 路由守卫：#capture 必须有确认目标
  useEffect(() => {
    const applyRoute = (h: ScreenId) => {
      if (h === 'capture') {
        const f = flowRef.current
        if (!canEnterCapture(f) && f.phase !== 'capturing' && f.phase !== 'completed') {
          showToast('请先完成发现与识别')
          navigate('discover', { replace: true })
          return
        }
      }
      if ((TAB_SCREENS as string[]).includes(h) || h === 'capture') {
        setScreen(h)
      }
    }
    const onHash = () => applyRoute(parseHashScreen())
    const onPop = () => applyRoute(parseHashScreen())
    window.addEventListener('hashchange', onHash)
    window.addEventListener('popstate', onPop)
    if (typeof location !== 'undefined' && !location.hash) {
      history.replaceState({ screen: 'discover' }, '', '#discover')
    }
    return () => {
      window.removeEventListener('hashchange', onHash)
      window.removeEventListener('popstate', onPop)
    }
  }, [navigate, showToast])

  useEffect(() => {
    if (screen !== 'capture' && flow.phase === 'capturing') {
      // 允许返回查看；不自动 reset
    }
  }, [screen, flow.phase])

  const handleInvalidCapture = useCallback(() => {
    dispatch({ type: 'RESET' })
    navigate('discover', { replace: true })
    showToast('捕获会话无效，已返回发现')
  }, [dispatch, navigate, showToast])

// Achievement entry only when flag + unlock — never toast-spam locked features
  const handleAchievement = useCallback(() => {
    if (!FEATURE_FLAGS.achievements) {
      showToast('成就暂未开放')
      return
    }
    if (!progression.isFeatureUnlocked('achievement')) return
    showToast(`等级 Lv.${level} · 经验 ${exp} · 成就进度开发中`)
  }, [progression, showToast, level, exp])

  const showReturnBanner = useMemo(() => {
    const { returnSummary, state } = progression
    if (!returnSummary.isReturning) return false
    if (state.returnBannerDismissedAt == null) return true
    return state.returnBannerDismissedAt <= state.lastActiveAt
  }, [progression])

  const handleNavigateWithProgress = useCallback(
    (next: ScreenId) => {
      if (next === 'pokedex') progression.openPokedex()
      if (next === 'map') {
        // Map open counts as free safe-explore activity
        progression.safeExplore()
      }
      navigate(next)
    },
    [navigate, progression],
  )
  const handleCoinsChange = useCallback(
    (next: number) => {
      const delta = next - gold
      if (delta > 0) addGold(delta)
    },
    [gold, addGold],
  )

  const cityLabel =
    lbs.state.cityName ||
    (lbs.state.geoStatus === 'locating'
      ? '定位中'
      : lbs.state.geoStatus === 'denied'
        ? '定位关闭'
        : '未知城市')

  const weatherLabel = weather.todayMeta
    ? `${weather.todayMeta.emoji}${weather.todayMeta.name}`
    : weather.state.status === 'loading'
      ? '天气…'
      : weather.state.source === 'internal'
        ? '本地天气'
        : '—'

  const discoverBlock = (
    <>
      <DiscoverScreen
        energy={currentStamina}
        coins={gold}
        flow={flow}
        dispatch={dispatch}
        onNavigate={handleNavigateWithProgress}
        onEnterCapture={handleEnterCapture}
        onOpenAccount={() => setShowAccount(true)}
        city={cityLabel}
        weather={weatherLabel}
      />
      <DailyGoalsPanel
        goals={progression.dailyGoals}
        returnSummary={progression.returnSummary}
        showReturnBanner={showReturnBanner}
        onDismissReturn={progression.dismissReturnBanner}
        onNavigate={handleNavigateWithProgress}
        onSafeExplore={progression.safeExplore}
        onSeasonCheckin={progression.seasonCheckin}
        staminaEmpty={currentStamina <= 0}
      />
    </>
  )

  const renderScreen = () => {
    if (showAccount) {
      return (
        <AccountSettingsPanel onToast={showToast} onClose={() => setShowAccount(false)} />
      )
    }
    switch (screen) {
      case 'discover':
        return discoverBlock
      case 'map':
        return (
          <HuntMapScreen
            selectedTargetId={selectedTargetId}
            onSelectTarget={setSelectedTargetId}
            onBack={() => navigate('discover', { replace: true })}
          />
        )
      case 'capture': {
        if (!flow.selectedBox || !flow.detectInferenceId || !flow.captureAttemptId) {
          queueMicrotask(() => handleInvalidCapture())
          return discoverBlock
        }
        return (
          <CaptureScreen
            onToast={showToast}
            species={flow.selectedBox.species}
            detection={flow.selectedBox}
            detectInferenceId={flow.detectInferenceId}
            photoBlob={flow.photoBlob}
            targetId={flow.targetId}
            captureAttemptId={flow.captureAttemptId}
            onInvalidAccess={handleInvalidCapture}
            onSettled={(ok) => {
              if (ok) {
                dispatch({ type: 'COMPLETE' })
                progression.recordCapture(true)
              } else {
                dispatch({ type: 'FAIL', code: 'capture_failed', message: '捕获失败' })
              }
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
      case 'settings':
        return <SettingsScreen onToast={showToast} />
      default:
        return null
    }
  }

    <div className="ap-root">
      <RouteAnnouncerElement />
      <OnboardingOverlay />
      <a className="ap-skip-link" href="#ap-main-content">
        跳到主要内容
      </a>
      <PhoneFrame variant={screen}>
        <div className="ap-main" id="ap-main-content" tabIndex={-1}>
          {renderScreen()}
        </div>
        {screen !== 'map' && (
          <BottomTabBar
            active={screen === 'capture' ? 'discover' : screen}
            onChange={handleNavigateWithProgress}
            onAchievement={
              progression.isFeatureUnlocked('achievement') ? handleAchievement : undefined
            }
            unlockedFeatures={progression.unlockedFeatures}
          />
        )}
        <div className={`ap-toast ${toastMessage ? 'is-visible' : ''}`} role="status" aria-live="polite">
          {toastMessage}
        </div>
      </PhoneFrame>
    </div>
  )
}
