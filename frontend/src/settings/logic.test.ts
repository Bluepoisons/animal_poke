import { describe, it, expect } from 'vitest'
import {
  normalizeSettings,
  loadSettingsFromStorage,
  saveSettingsToStorage,
  pickSyncableSettings,
  exportSettingsJson,
} from './logic'
import { DEFAULT_USER_SETTINGS, SETTINGS_STORAGE_KEY } from './types'

describe('settings logic', () => {
  it('normalizes partial / invalid input', () => {
    expect(normalizeSettings(null)).toEqual(DEFAULT_USER_SETTINGS)
    expect(normalizeSettings({ locale: 'en', sfxEnabled: false }).locale).toBe('en')
    expect(normalizeSettings({ locale: 'xx' as 'zh' }).locale).toBe('zh')
  })

  it('persists and loads from storage adapter', () => {
    const store: Record<string, string> = {}
    const settings = { ...DEFAULT_USER_SETTINGS, musicEnabled: false, locale: 'en' as const }
    saveSettingsToStorage(settings, (k, v) => {
      store[k] = v
    })
    expect(store[SETTINGS_STORAGE_KEY]).toBeTruthy()
    const loaded = loadSettingsFromStorage((k) => store[k] ?? null)
    expect(loaded.musicEnabled).toBe(false)
    expect(loaded.locale).toBe('en')
  })

  it('pickSyncableSettings omits when sync disabled', () => {
    expect(pickSyncableSettings({ ...DEFAULT_USER_SETTINGS, syncNonSensitive: false })).toEqual({})
    const sync = pickSyncableSettings(DEFAULT_USER_SETTINGS)
    expect(sync.locale).toBe('zh')
    expect(sync).not.toHaveProperty('syncNonSensitive')
  })

  it('exportSettingsJson is valid JSON with version', () => {
    const raw = exportSettingsJson(DEFAULT_USER_SETTINGS)
    const parsed = JSON.parse(raw)
    expect(parsed.version).toBe(1)
    expect(parsed.settings.locale).toBe('zh')
  })
})
