import {
  DEFAULT_USER_SETTINGS,
  SETTINGS_STORAGE_KEY,
  SYNCABLE_SETTING_KEYS,
  type UserSettings,
} from './types'
import type { Locale } from '../i18n'

export function isLocale(v: unknown): v is Locale {
  return v === 'zh' || v === 'en'
}

export function normalizeSettings(raw: Partial<UserSettings> | null | undefined): UserSettings {
  const base = { ...DEFAULT_USER_SETTINGS }
  if (!raw || typeof raw !== 'object') return base
  if (isLocale(raw.locale)) base.locale = raw.locale
  if (typeof raw.sfxEnabled === 'boolean') base.sfxEnabled = raw.sfxEnabled
  if (typeof raw.musicEnabled === 'boolean') base.musicEnabled = raw.musicEnabled
  if (typeof raw.hapticsEnabled === 'boolean') base.hapticsEnabled = raw.hapticsEnabled
  if (typeof raw.reduceMotion === 'boolean') base.reduceMotion = raw.reduceMotion
  if (typeof raw.dataSaver === 'boolean') base.dataSaver = raw.dataSaver
  if (typeof raw.syncNonSensitive === 'boolean') base.syncNonSensitive = raw.syncNonSensitive
  if (typeof raw.homeMode === 'boolean') base.homeMode = raw.homeMode
  return base
}

export function loadSettingsFromStorage(
  getItem: (k: string) => string | null = (k) => {
    try {
      return localStorage.getItem(k)
    } catch {
      return null
    }
  },
): UserSettings {
  try {
    const raw = getItem(SETTINGS_STORAGE_KEY)
    if (!raw) return { ...DEFAULT_USER_SETTINGS }
    return normalizeSettings(JSON.parse(raw) as Partial<UserSettings>)
  } catch {
    return { ...DEFAULT_USER_SETTINGS }
  }
}

export function saveSettingsToStorage(
  settings: UserSettings,
  setItem: (k: string, v: string) => void = (k, v) => {
    try {
      localStorage.setItem(k, v)
    } catch { /* ignore */ }
  },
): void {
  setItem(SETTINGS_STORAGE_KEY, JSON.stringify(settings))
}

/** Payload safe to push after account bind */
export function pickSyncableSettings(settings: UserSettings): Partial<UserSettings> {
  if (!settings.syncNonSensitive) return {}
  const out: Partial<UserSettings> = {}
  for (const key of SYNCABLE_SETTING_KEYS) {
    // @ts-expect-error index
    out[key] = settings[key]
  }
  return out
}

export function exportSettingsJson(settings: UserSettings): string {
  return JSON.stringify(
    {
      version: 1,
      exportedAt: new Date().toISOString(),
      settings: pickSyncableSettings({ ...settings, syncNonSensitive: true }),
    },
    null,
    2,
  )
}
