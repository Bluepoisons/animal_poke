/**
 * 本机清理：设置 / 同意 / 分析 / 鉴权 / 本地图鉴（设备或账号删除后的客户端降级）。
 */

import { clearAuth, clearRefreshToken } from '../auth/deviceAuth'
import { AnimalRepository } from '../db/repositories/animal-repository'
import { revokeConsent } from '../compliance'
import { clearAnalyticsPref } from './analyticsPrefs'
import { onAnalyticsConsentRevoked } from '../analytics'
import { SETTINGS_STORAGE_KEY } from '../settings/types'

const CONSENT_KEY = 'animal-poke-consent'
const AGE_KEY = 'animal-poke-age-verification'
const PENDING_KEY = 'animal-poke-consent-pending'
const LOCALE_KEY = 'animal-poke-locale'
const ACCOUNT_ID_KEY = 'ap_account_id'

export type LocalClearLevel = 'settings' | 'device' | 'account'

function safeRemove(key: string): void {
  try {
    localStorage.removeItem(key)
  } catch {
    /* ignore */
  }
}

/** 仅清除本机设置偏好（不删图鉴/同意/鉴权） */
export async function clearLocalSettingsOnly(): Promise<void> {
  safeRemove(SETTINGS_STORAGE_KEY)
  safeRemove(LOCALE_KEY)
}

/** 设备级删除后的本地清理：同意、分析、图鉴、token */
export async function clearLocalAfterDeviceDelete(): Promise<void> {
  await clearLocalSettingsOnly()
  safeRemove(CONSENT_KEY)
  safeRemove(AGE_KEY)
  safeRemove(PENDING_KEY)
  clearAnalyticsPref()
  onAnalyticsConsentRevoked()
  revokeConsent()
  clearAuth()
  clearRefreshToken()
  try {
    const all = await AnimalRepository.getAll()
    await Promise.all(all.map((a) => AnimalRepository.delete(a.id)))
  } catch {
    /* IDB 不可用时忽略 */
  }
}

/** 账号注销后：设备清理 + 账号 id */
export async function clearLocalAfterAccountDelete(): Promise<void> {
  await clearLocalAfterDeviceDelete()
  safeRemove(ACCOUNT_ID_KEY)
}

export async function applyLocalClear(level: LocalClearLevel): Promise<void> {
  if (level === 'settings') {
    await clearLocalSettingsOnly()
    return
  }
  if (level === 'account') {
    await clearLocalAfterAccountDelete()
    return
  }
  await clearLocalAfterDeviceDelete()
}
