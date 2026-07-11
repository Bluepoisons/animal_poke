/**
 * 分析偏好（本地 only，不进入服务端 consent scope）。
 */

const KEY = 'animal-poke-analytics-pref'

export type AnalyticsPref = 'granted' | 'denied' | 'unset'

export function getAnalyticsPref(
  getItem: (k: string) => string | null = (k) => {
    try {
      return localStorage.getItem(k)
    } catch {
      return null
    }
  },
): AnalyticsPref {
  try {
    const v = getItem(KEY)
    if (v === 'granted' || v === 'denied') return v
    return 'unset'
  } catch {
    return 'unset'
  }
}

export function setAnalyticsPref(
  pref: 'granted' | 'denied',
  setItem: (k: string, v: string) => void = (k, v) => {
    try {
      localStorage.setItem(k, v)
    } catch {
      /* ignore */
    }
  },
): void {
  setItem(KEY, pref)
}

export function clearAnalyticsPref(
  removeItem: (k: string) => void = (k) => {
    try {
      localStorage.removeItem(k)
    } catch {
      /* ignore */
    }
  },
): void {
  removeItem(KEY)
}

export const ANALYTICS_PREF_KEY = KEY
