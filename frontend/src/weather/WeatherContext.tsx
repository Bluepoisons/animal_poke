import React, { createContext, useReducer, useEffect, useCallback, useMemo } from 'react'
import { useLbs } from '../lbs/useLbs'
import {
  WEATHER_CACHE_KEY,
} from './constants'
import type {
  WeatherState, WeatherAction, WeatherContextValue,
  WeekWeather, CurrentWeather, WeatherMeta, WeatherType,
  CaptureModifierResult, ColdCheckResult,
} from './types'
import { WEATHER_META } from './types'
import {
  generateWeekWeather, getTodayWeather, getWeekStart,
  getCaptureModifier, getColdRisk,
} from './logic'
import { fetchWeekWeather } from './api'

/** 默认初始状态 */
const defaultState: WeatherState = {
  week: null,
  weekStart: 0,
  city: '',
  source: 'internal',
  status: 'idle',
  errorMsg: '',
}

/** 从 localStorage 加载缓存天气 */
function loadInitialState(): WeatherState {
  try {
    const saved = localStorage.getItem(WEATHER_CACHE_KEY)
    if (saved) {
      const parsed = JSON.parse(saved) as WeatherState
      // 检查缓存是否过期（超过一周）
      const cachedWeekStart = parsed.weekStart
      const currentWeekStart = getWeekStart()
      if (cachedWeekStart === currentWeekStart && parsed.week) {
        // 缓存仍然有效（同一周）
        return {
          ...parsed,
          status: 'loaded',
          errorMsg: '',
        }
      }
      // 缓存过期，但保留 city 信息
      if (parsed.city) {
        return { ...defaultState, city: parsed.city }
      }
    }
  } catch (e) {
    console.warn('加载天气缓存失败:', e)
  }
  return defaultState
}

/** Reducer */
function weatherReducer(state: WeatherState, action: WeatherAction): WeatherState {
  switch (action.type) {
    case 'LOADING':
      return { ...state, status: 'loading', errorMsg: '' }
    case 'SET_WEEK':
      return {
        ...state,
        week: action.week,
        weekStart: action.weekStart,
        city: action.city,
        source: action.source,
        status: 'loaded',
        errorMsg: '',
      }
    case 'ERROR':
      return { ...state, status: 'error', errorMsg: action.msg }
    case 'RESET':
      return defaultState
    default:
      return state
  }
}

export const WeatherContext = createContext<WeatherContextValue | null>(null)

export const WeatherProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [state, dispatch] = useReducer(weatherReducer, undefined, loadInitialState)
  const lbs = useLbs() // 需放在 LbsProvider 内

  const cityName = lbs.state.cityName
  const playerLocation = lbs.state.playerLocation

  /** 为指定城市生成本周天气 */
  const loadWeather = useCallback(async (city: string) => {
    if (!city) return

    const weekStart = getWeekStart()
    dispatch({ type: 'LOADING' })

    // 优先用 lat/lng 调后端；失败则确定性本地降级
    const lat = playerLocation?.lat
    const lng = playerLocation?.lng
    if (typeof lat === 'number' && typeof lng === 'number') {
      const backend = await fetchWeekWeather(lat, lng)
      if (backend) {
        dispatch({ type: 'SET_WEEK', week: backend.week, weekStart, city, source: 'backend' })
        return
      }
    }

    // 降级为游戏内随机生成（仅接口异常时）
    const week = generateWeekWeather(city, weekStart)
    dispatch({ type: 'SET_WEEK', week, weekStart, city, source: 'internal' })
  }, [playerLocation?.lat, playerLocation?.lng])

  // 城市名变化时自动加载天气
  useEffect(() => {
    if (cityName && cityName !== state.city) {
      loadWeather(cityName)
    }
  }, [cityName, state.city, loadWeather])

  // 周日 0:00 定时刷新检查（每分钟检查一次）
  useEffect(() => {
    const checkWeekReset = () => {
      const currentWeekStart = getWeekStart()
      if (state.weekStart && currentWeekStart !== state.weekStart) {
        // 新的一周，重新加载
        loadWeather(state.city)
      }
    }

    const interval = setInterval(checkWeekReset, 60_000)
    return () => clearInterval(interval)
  }, [state.weekStart, state.city, loadWeather])

  // 页面可见性恢复时检查周切换
  useEffect(() => {
    const handleVisibility = () => {
      if (document.visibilityState === 'visible') {
        const currentWeekStart = getWeekStart()
        if (state.weekStart && currentWeekStart !== state.weekStart && state.city) {
          loadWeather(state.city)
        }
      }
    }
    document.addEventListener('visibilitychange', handleVisibility)
    return () => document.removeEventListener('visibilitychange', handleVisibility)
  }, [state.weekStart, state.city, loadWeather])

  // localStorage 持久化
  useEffect(() => {
    if (state.status === 'loaded' && state.week) {
      localStorage.setItem(WEATHER_CACHE_KEY, JSON.stringify(state))
    }
  }, [state])

  // 今日天气（派生状态，使用 useMemo 避免对象引用变化）
  const today = useMemo<CurrentWeather | null>(() => {
    return getTodayWeather(state.week)
  }, [state.week])

  const todayMeta = useMemo<WeatherMeta | null>(() => {
    return today ? WEATHER_META[today.weather] : null
  }, [today])

  // 派生计算函数
  const captureModifierFn = useCallback((): CaptureModifierResult => {
    if (!today) return { modifier: 0, multiplier: 1.0, description: '天气数据加载中' }
    return getCaptureModifier(today.weather)
  }, [today])

  const coldRiskFn = useCallback((): ColdCheckResult => {
    if (!today) return { isRisky: false, probability: 0, description: '天气数据加载中' }
    return getColdRisk(today.weather)
  }, [today])

  const battleModifierFn = useCallback((): WeatherType => {
    return today?.weather ?? 'sunny'
  }, [today])

  const refreshWeather = useCallback((city: string) => {
    loadWeather(city)
  }, [loadWeather])

  const value = useMemo<WeatherContextValue>(() => ({
    state,
    today,
    todayMeta,
    getCaptureModifier: captureModifierFn,
    getColdRisk: coldRiskFn,
    getBattleModifier: battleModifierFn,
    refreshWeather,
  }), [state, today, todayMeta, captureModifierFn, coldRiskFn, battleModifierFn, refreshWeather])

  return (
    <WeatherContext.Provider value={value}>
      {children}
    </WeatherContext.Provider>
  )
}
