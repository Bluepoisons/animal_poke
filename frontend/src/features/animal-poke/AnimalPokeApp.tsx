import { useState, useCallback, useRef, useEffect, useReducer, useMemo } from 'react'
import { flushSync } from 'react-dom'
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
import JournalScreen from './screens/JournalScreen'
import PrologueScreen from './screens/PrologueScreen'
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
import { RouteAnnouncerElement, useRouteAnnouncer } from '../../a11y'
import OnboardingOverlay from './components/OnboardingOverlay'
import { setCaptureActive } from '../../pwa/updateGate'

const TAB_SCREENS: ScreenId[] = ['discover', 'map', 'pokedex', 'journal', 'battle', 'store', 'settings']

const SCREEN_FEATURES: Partial<Record<ScreenId, FeatureId>> = {
  discover: 'discover',
  map: 'map',
  pokedex: 'pokedex',
  battle: 'battle',
  store: 'store',
}

const ROUTE_TITLES: Record<ScreenId, string> = {
  discover: '发现',
  map: '猎取地图',
  capture: '捕获',
  pokedex: '图鉴',
  battle: '对战竞技场',
  store: '商店',
  settings: '设置',
  journal: '城市手账',
  prologue: '序章',
}

function parseHashScreen(): ScreenId {
  const h = (typeof location !== 'undefined' ? location.hash.replace('#', '') : '') as ScreenId
  const allowed: ScreenId[] = ['discover', 'map', 'capture', 'pokedex', 'journal', 'prologue', 'battle', 'store', 'settings']
  return allowed.includes(h) ? h : 'discover'
}

export function newAttemptId(): string {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) return crypto.randomUUID()
  const bytes = new Uint8Array(16)
  for (let i = 0; i < bytes.length; i += 1) bytes[i] = Math.floor(Math.random() * 256)
  bytes[6] = (bytes[6] & 0x0f) | 0x40
  bytes[8] = (bytes[8] & 0x3f) | 0x80
  const hex = Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('')
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
}

function isScreenUnlocked(
  screen: ScreenId,
  checkFeature: (feature: FeatureId) => boolean,
): boolean {
  const feature = SCREEN_FEATURES[screen]
  return feature ? checkFeature(feature) : true
}

export default function AnimalPokeApp() {
  const progression = useProgression()
  const [screen, setScreen] = useState<ScreenId>(() => {
    const h = parseHashScreen()
    // 直接访问 #capture 无状态时回 discover
    if (h === 'capture') return 'discover'
    return isScreenUnlocked(h, progression.isFeatureUnlocked) ? h : 'discover'
  })
  const [selectedTargetId, setSelectedTargetId] = useState('target-uncommon-50')
  const { state: staminaState, addGold } = useStamina()
  const level = staminaState.level
  const exp = staminaState.exp
  const lbs = useLbs()
  const weather = useWeather()
  const currentStamina = staminaState.currentStamina
  const gold = staminaState.gold
  const [toastMessage, setToastMessage] = useState<string | null>(null)
  const [showAccount, setShowAccount] = useState(false)
  useRouteAnnouncer(showAccount ? '账户设置' : ROUTE_TITLES[screen])
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
      if (!isScreenUnlocked(nextScreen, progression.isFeatureUnlocked)) return
      setScreen(nextScreen)
      if (typeof history === 'undefined') return
      const url = `#${nextScreen}`
      if (opts?.replace) history.replaceState({ screen: nextScreen }, '', url)
      else history.pushState({ screen: nextScreen }, '', url)
    },
    [progression],
  )

  const handleEnterCapture = useCallback(() => {
    let f = flowRef.current
    if (!f.selectedBox || !f.detectInferenceId || !f.photoBlob) {
      showToast('识别数据不完整')
      navigate('discover', { replace: true })
      return
    }
    // Build the full post-enter snapshot offline, then HYDRATE + setScreen in one
    // flushSync so CaptureScreen never mounts on a lagging reducer state.
    if (f.phase !== 'target_confirmed' && f.phase !== 'capturing') {
      f = reduceCaptureFlow(f, { type: 'CONFIRM_TARGET' })
      if (f.phase === 'failed') {
        showToast(f.errorMessage || '无法确认目标')
        return
      }
    }
    const attemptId = newAttemptId()
    f = reduceCaptureFlow(f, { type: 'ENTER_CAPTURE', attemptId })
    if (f.phase === 'failed' || !f.captureAttemptId || !f.selectedBox) {
      showToast(f.errorMessage || '无法进入捕获')
      return
    }
    flowRef.current = f
    flushSync(() => {
      dispatchFlow({ type: 'HYDRATE', state: f })
      setScreen('capture')
    })
    flowRef.current = f
    if (typeof history !== 'undefined') {
      history.pushState({ screen: 'capture' }, '', '#capture')
    }
  }, [showToast, navigate])

  // 路由守卫：#capture 必须有确认目标
  useEffect(() => {
    const applyRoute = (h: ScreenId) => {
      if (h === 'capture') {
        const f = flowRef.current
        const ready =
          !!f.captureAttemptId &&
          !!f.selectedBox &&
          !!f.detectInferenceId &&
          (canEnterCapture(f) ||
            f.phase === 'capturing' ||
            f.phase === 'completed' ||
            f.phase === 'target_confirmed')
        if (!ready) {
          showToast('请先完成发现与识别')
          navigate('discover', { replace: true })
          return
        }
      }
      if (!isScreenUnlocked(h, progression.isFeatureUnlocked)) {
        navigate('discover', { replace: true })
        return
      }
      if ((TAB_SCREENS as string[]).includes(h) || h === 'capture') {
        setScreen(h)
      }
    }
    const onHash = () => applyRoute(parseHashScreen())
    const onPop = () => applyRoute(parseHashScreen())
    window.addEventListener('hashchange', onHash)
    window.addEventListener('popstate', onPop)
    if (typeof location !== 'undefined' && !location.hash) navigate('discover', { replace: true })
    else onHash()
    return () => {
      window.removeEventListener('hashchange', onHash)
      window.removeEventListener('popstate', onPop)
    }
  }, [navigate, progression.isFeatureUnlocked, showToast])

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
        // Prefer flowRef: ENTER_CAPTURE updates the ref before navigate, while
        // useReducer state may lag one paint (E2E enter-capture race).
        let cap = flowRef.current.captureAttemptId
          ? flowRef.current
          : flow.captureAttemptId
            ? flow
            : flowRef.current.selectedBox
              ? flowRef.current
              : flow
        if (!cap.selectedBox || !cap.detectInferenceId || !cap.photoBlob) {
          queueMicrotask(() => handleInvalidCapture())
          return discoverBlock
        }
        // Last-resort: materialize attempt id if navigation won the race
        if (!cap.captureAttemptId) {
          const attemptId = newAttemptId()
          cap = reduceCaptureFlow(cap, { type: 'ENTER_CAPTURE', attemptId })
          flowRef.current = cap
          queueMicrotask(() => dispatchFlow({ type: 'ENTER_CAPTURE', attemptId }))
        }
        const selected = cap.selectedBox
        const inferenceId = cap.detectInferenceId
        const attemptId = cap.captureAttemptId
        if (!selected || !inferenceId || !attemptId) {
          queueMicrotask(() => handleInvalidCapture())
          return discoverBlock
        }
        return (
          <CaptureScreen
            onToast={showToast}
            species={selected.species}
            detection={selected}
            detectInferenceId={inferenceId}
            photoBlob={cap.photoBlob}
            targetId={cap.targetId}
            captureAttemptId={attemptId}
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
      case 'journal':
        return <JournalScreen onToast={showToast} onOpenPrologue={() => navigate('prologue')} />
      case 'prologue':
        return (
          <PrologueScreen
            onToast={showToast}
            onFinished={() => navigate('journal')}
          />
        )
      case 'battle':
        return <BattleArenaScreen />
      case 'store':
        return <StoreScreen coins={gold} onCoinsChange={handleCoinsChange} onToast={showToast} />
      case 'settings':
        return (
          <SettingsScreen
            onToast={showToast}
            onOpenAccount={() => setShowAccount(true)}
          />
        )
      default:
        return null
    }
  }

  return (
    <div className="ap-root">
      <RouteAnnouncerElement />
      <OnboardingOverlay />
      <a className="ap-skip-link" href="#ap-main-content">
        跳到主要内容
      </a>
      <PhoneFrame variant={screen}>
        <main className="ap-main" id="ap-main-content" tabIndex={-1}>
          {renderScreen()}
        </main>
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
