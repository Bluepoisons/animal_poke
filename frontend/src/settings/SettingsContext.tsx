import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react'
import { useI18n, type Locale } from '../i18n'
import {
  loadSettingsFromStorage,
  saveSettingsToStorage,
  pickSyncableSettings,
  exportSettingsJson,
  normalizeSettings,
} from './logic'
import { DEFAULT_USER_SETTINGS, type UserSettings } from './types'
import { SettingsRepository } from '../db/repositories/settings-repository'

interface SettingsContextValue {
  settings: UserSettings
  update: (patch: Partial<UserSettings>) => void
  exportData: () => string
  deleteLocalData: () => Promise<void>
  /** Non-sensitive prefs for post-bind sync */
  getSyncPayload: () => Partial<UserSettings>
}

const SettingsContext = createContext<SettingsContextValue | null>(null)

export function SettingsProvider({ children }: { children: ReactNode }) {
  const { locale, setLocale } = useI18n()
  const [settings, setSettings] = useState<UserSettings>(() => {
    const loaded = loadSettingsFromStorage()
    return { ...loaded, locale: loaded.locale || locale }
  })

  // Keep i18n locale in sync
  useEffect(() => {
    if (settings.locale !== locale) {
      setLocale(settings.locale)
    }
  }, [settings.locale, locale, setLocale])

  // Persist localStorage + IndexedDB (non-sensitive subset)
  useEffect(() => {
    saveSettingsToStorage(settings)
    void SettingsRepository.save({
      key: 'prefs',
      soundEnabled: settings.sfxEnabled,
      musicEnabled: settings.musicEnabled,
      privacyConsented: true,
      hapticsEnabled: settings.hapticsEnabled,
      reduceMotion: settings.reduceMotion,
      dataSaver: settings.dataSaver,
      locale: settings.locale,
    }).catch(() => {})
  }, [settings])

  // Respect prefers-reduced-motion when reduceMotion on
  useEffect(() => {
    document.documentElement.dataset.reduceMotion = settings.reduceMotion ? '1' : '0'
    document.documentElement.dataset.dataSaver = settings.dataSaver ? '1' : '0'
  }, [settings.reduceMotion, settings.dataSaver])

  const update = useCallback((patch: Partial<UserSettings>) => {
    setSettings((prev) => {
      const next = normalizeSettings({ ...prev, ...patch })
      if (patch.locale) {
        // locale applied via effect → setLocale
      }
      return next
    })
  }, [])

  const exportData = useCallback(() => exportSettingsJson(settings), [settings])

  const deleteLocalData = useCallback(async () => {
    try {
      localStorage.removeItem('animal_poke_user_settings')
      localStorage.removeItem('animal-poke-locale')
    } catch { /* ignore */ }
    setSettings({ ...DEFAULT_USER_SETTINGS })
    await SettingsRepository.save({
      key: 'prefs',
      soundEnabled: true,
      musicEnabled: true,
      privacyConsented: true,
    }).catch(() => {})
  }, [])

  const getSyncPayload = useCallback(
    () => pickSyncableSettings(settings),
    [settings],
  )

  const value = useMemo(
    () => ({ settings, update, exportData, deleteLocalData, getSyncPayload }),
    [settings, update, exportData, deleteLocalData, getSyncPayload],
  )

  return <SettingsContext.Provider value={value}>{children}</SettingsContext.Provider>
}

export function useSettings(): SettingsContextValue {
  const ctx = useContext(SettingsContext)
  if (!ctx) throw new Error('useSettings must be used within SettingsProvider')
  return ctx
}

export type { Locale }
