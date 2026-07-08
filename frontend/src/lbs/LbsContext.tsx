import React, { createContext, useReducer, useEffect, useCallback, useMemo, useRef } from 'react'
import {
  REFRESH_INTERVAL_MS,
  GEOLOCATION_OPTIONS,
  LBS_CACHE_KEY,
} from './constants'
import type { LbsState, LbsAction, LbsContextValue, GeoLocation, DiscoveryPoint } from './types'
import { generateDiscoveryPoints, filterExpired, updatePointStatus, shouldRefresh } from './logic'

/** 默认初始状态 */
const defaultState: LbsState = {
  geoStatus: 'idle',
  playerLocation: null,
  cityName: '',
  provinceName: '',
  discoveryPoints: [],
  lastRefreshTime: 0,
  errorMsg: '',
}

/** 从 localStorage 加载缓存的位置和城市名（发现点不缓存） */
function loadInitialState(): LbsState {
  try {
    const saved = localStorage.getItem(LBS_CACHE_KEY)
    if (saved) {
      const parsed = JSON.parse(saved)
      // 基本字段校验
      if (parsed.playerLocation && typeof parsed.playerLocation.lat === 'number' && typeof parsed.playerLocation.lng === 'number') {
        return {
          ...defaultState,
          playerLocation: parsed.playerLocation as GeoLocation,
          cityName: parsed.cityName ?? '',
          provinceName: parsed.provinceName ?? '',
          geoStatus: 'located',
        }
      }
    }
  } catch (e) {
    console.warn('加载 LBS 缓存失败，使用默认值:', e)
  }
  return defaultState
}

/** 调用后端逆地理编码获取城市名（静默降级） */
async function fetchCityName(lat: number, lng: number): Promise<{ city: string; province: string }> {
  try {
    const resp = await fetch(`/api/v1/geo/city?lat=${lat}&lng=${lng}`)
    if (!resp.ok) throw new Error(`geo/city 请求失败: ${resp.status}`)
    const data = await resp.json()
    return {
      city: data.city ?? '',
      province: data.province ?? '',
    }
  } catch {
    // 静默降级：不抛出错误，返回空值
    return { city: '', province: '' }
  }
}

/** Reducer */
function lbsReducer(state: LbsState, action: LbsAction): LbsState {
  switch (action.type) {
    case 'GEO_START':
      return { ...state, geoStatus: 'locating', errorMsg: '' }

    case 'GEO_SUCCESS':
      return {
        ...state,
        geoStatus: 'located',
        playerLocation: action.location,
        errorMsg: '',
      }

    case 'GEO_FAIL':
      return {
        ...state,
        geoStatus: action.status,
        errorMsg: action.errorMsg,
      }

    case 'GEO_CITY_SUCCESS':
      return {
        ...state,
        cityName: action.city,
        provinceName: action.province,
      }

    case 'GEO_CITY_FAIL':
      // 城市名获取失败时静默降级，不显示错误
      return state

    case 'REFRESH_POINTS':
      return {
        ...state,
        discoveryPoints: action.points,
        lastRefreshTime: action.now,
      }

    case 'REMOVE_POINT':
      return {
        ...state,
        discoveryPoints: state.discoveryPoints.filter(p => p.id !== action.id),
      }

    case 'CLEAR_EXPIRED':
      return {
        ...state,
        discoveryPoints: filterExpired(state.discoveryPoints, action.now),
      }

    case 'LOAD_STATE':
      return { ...state, ...action.state }

    default:
      return state
  }
}

export const LbsContext = createContext<LbsContextValue | null>(null)

export const LbsProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [state, dispatch] = useReducer(lbsReducer, undefined, loadInitialState)
  const refreshTimerRef = useRef<ReturnType<typeof setInterval> | null>(null)

  /** 请求定位 */
  const requestLocation = useCallback(() => {
    if (!navigator.geolocation) {
      dispatch({ type: 'GEO_FAIL', status: 'unsupported', errorMsg: '设备不支持定位' })
      return
    }
    dispatch({ type: 'GEO_START' })
    navigator.geolocation.getCurrentPosition(
      (pos) => {
        dispatch({
          type: 'GEO_SUCCESS',
          location: {
            lat: pos.coords.latitude,
            lng: pos.coords.longitude,
            accuracy: pos.coords.accuracy,
          },
        })
        // 获取到位置后请求城市名（静默降级）
        fetchCityName(pos.coords.latitude, pos.coords.longitude)
          .then(res => dispatch({ type: 'GEO_CITY_SUCCESS', city: res.city, province: res.province }))
          .catch(() => dispatch({ type: 'GEO_CITY_FAIL' }))
      },
      (err) => {
        const status = err.code === 1 ? 'denied' : err.code === 3 ? 'timeout' : 'denied'
        dispatch({ type: 'GEO_FAIL', status, errorMsg: err.message })
      },
      GEOLOCATION_OPTIONS,
    )
  }, [])

  /** 手动刷新发现点 */
  const refreshPoints = useCallback(() => {
    if (!state.playerLocation) return
    const now = Date.now()
    // 过滤掉已过期的发现点
    const existing = filterExpired(state.discoveryPoints, now)
    // 更新所有现存发现点的距离状态
    const updated = updatePointStatus(existing, state.playerLocation)
    // 补充新发现点（至少保持 5 个，最多 8 个）
    const need = Math.max(0, 5 - updated.length)
    if (need === 0 && updated.length <= 8) {
      dispatch({ type: 'REFRESH_POINTS', points: updated, now })
      return
    }
    // 生成新发现点
    const newCount = need + 3 // 多生成几个，让玩家有新鲜感
    const newPoints = generateDiscoveryPoints(state.playerLocation, newCount, now)
    const allPoints = [...updated, ...newPoints]
      .slice(0, 8) // 同时存在上限 8 个
    // 再次更新距离状态（含新生成的点）
    const finalPoints = updatePointStatus(allPoints, state.playerLocation)
    dispatch({ type: 'REFRESH_POINTS', points: finalPoints, now })
  }, [state.playerLocation, state.discoveryPoints])

  /** 首次定位成功后自动刷新发现点 */
  useEffect(() => {
    if (state.geoStatus === 'located' && state.playerLocation && state.discoveryPoints.length === 0) {
      refreshPoints()
    }
  }, [state.geoStatus, state.playerLocation, state.discoveryPoints.length, refreshPoints])

  /** 定时刷新发现点（每 5 分钟） */
  useEffect(() => {
    if (state.geoStatus !== 'located' || !state.playerLocation) {
      if (refreshTimerRef.current) {
        clearInterval(refreshTimerRef.current)
        refreshTimerRef.current = null
      }
      return
    }
    // 避免重复创建定时器
    if (refreshTimerRef.current) return
    refreshTimerRef.current = setInterval(() => {
      const now = Date.now()
      if (shouldRefresh(state.lastRefreshTime, now, REFRESH_INTERVAL_MS)) {
        refreshPoints()
      }
    }, REFRESH_INTERVAL_MS)
    return () => {
      if (refreshTimerRef.current) {
        clearInterval(refreshTimerRef.current)
        refreshTimerRef.current = null
      }
    }
  }, [state.geoStatus, state.playerLocation, state.lastRefreshTime, refreshPoints])

  /** 定时清理过期发现点（每 30 秒） */
  useEffect(() => {
    const interval = setInterval(() => {
      dispatch({ type: 'CLEAR_EXPIRED', now: Date.now() })
    }, 30_000)
    return () => clearInterval(interval)
  }, [])

  /** 定位位置刷新（每 30 秒） */
  useEffect(() => {
    if (state.geoStatus !== 'located') return
    const interval = setInterval(() => {
      requestLocation()
    }, 30_000)
    return () => clearInterval(interval)
  }, [state.geoStatus, requestLocation])

  /** 页面可见性恢复时刷新位置 */
  useEffect(() => {
    const handleVisibility = () => {
      if (document.visibilityState === 'visible') {
        requestLocation()
      }
    }
    document.addEventListener('visibilitychange', handleVisibility)
    return () => document.removeEventListener('visibilitychange', handleVisibility)
  }, [requestLocation])

  /** 缓存位置和城市名到 localStorage（发现点不缓存） */
  useEffect(() => {
    if (state.playerLocation) {
      localStorage.setItem(LBS_CACHE_KEY, JSON.stringify({
        playerLocation: state.playerLocation,
        cityName: state.cityName,
        provinceName: state.provinceName,
      }))
    }
  }, [state.playerLocation, state.cityName, state.provinceName])

  /** 移除发现点（捕获成功后调用） */
  const removePoint = useCallback((id: string) => {
    dispatch({ type: 'REMOVE_POINT', id })
  }, [])

  /** 获取玩家附近的 in_range 发现点 */
  const getInRangePoints = useCallback((): DiscoveryPoint[] => {
    return state.discoveryPoints.filter(p => p.status === 'in_range')
  }, [state.discoveryPoints])

  /** 距下次刷新的秒数 */
  const nextRefreshIn = useMemo(() => {
    if (state.lastRefreshTime === 0) return 0
    const elapsed = Date.now() - state.lastRefreshTime
    const remaining = Math.max(0, REFRESH_INTERVAL_MS - elapsed)
    return Math.ceil(remaining / 1000)
  }, [state.lastRefreshTime])

  const value = useMemo<LbsContextValue>(() => ({
    state,
    requestLocation,
    refreshPoints,
    removePoint,
    getInRangePoints,
    nextRefreshIn,
  }), [state, requestLocation, refreshPoints, removePoint, getInRangePoints, nextRefreshIn])

  return (
    <LbsContext.Provider value={value}>
      {children}
    </LbsContext.Provider>
  )
}
