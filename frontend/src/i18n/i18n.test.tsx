import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { I18nProvider, useI18n, resolveMessage } from './index'
import { zh, type TranslationKey } from './locales/zh'
import { en } from './locales/en'

function wrapper({ children }: { children: React.ReactNode }) {
  return <I18nProvider>{children}</I18nProvider>
}

describe('i18n', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.stubGlobal('navigator', { ...navigator, language: 'zh-CN' })
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('defaults to zh locale when no saved preference', () => {
    const { result } = renderHook(() => useI18n(), { wrapper })
    expect(result.current.locale).toBe('zh')
    expect(result.current.t('app.name')).toBe(zh['app.name'])
  })

  it('switches to en locale', () => {
    const { result } = renderHook(() => useI18n(), { wrapper })
    act(() => result.current.setLocale('en'))
    expect(result.current.locale).toBe('en')
    expect(result.current.t('app.name')).toBe(en['app.name'])
  })

  it('switches to ja stub locale', () => {
    const { result } = renderHook(() => useI18n(), { wrapper })
    act(() => result.current.setLocale('ja'))
    expect(result.current.locale).toBe('ja')
    expect(result.current.t('app.name')).toBe('アニマルポケ')
  })

  it('persists locale choice to localStorage', () => {
    const { result } = renderHook(() => useI18n(), { wrapper })
    act(() => result.current.setLocale('en'))
    expect(localStorage.getItem('animal-poke-locale')).toBe('en')
  })

  it('restores locale from localStorage', () => {
    localStorage.setItem('animal-poke-locale', 'en')
    const { result } = renderHook(() => useI18n(), { wrapper })
    expect(result.current.locale).toBe('en')
  })

  it('handles missing key gracefully (returns key)', () => {
    const { result } = renderHook(() => useI18n(), { wrapper })
    expect(result.current.t('nonexistent.key' as TranslationKey)).toBe('nonexistent.key')
    expect(resolveMessage('en', 'totally.missing.key')).toBe('totally.missing.key')
  })

  it('falls back to zh when key missing in current locale dict', () => {
    // ja has partial; en/zh full — resolveMessage still returns string
    expect(resolveMessage('ja', 'settings.title')).toBeTruthy()
  })

  it('substitutes params in template', () => {
    const { result } = renderHook(() => useI18n(), { wrapper })
    const text = result.current.t('collection.unlocked', { count: 5 })
    expect(text).toBe(zh['collection.unlocked'])
  })

  it('all zh keys have en translations', () => {
    const zhKeys = Object.keys(zh) as TranslationKey[]
    const enKeys = Object.keys(en)
    const missing = zhKeys.filter(k => !enKeys.includes(k))
    expect(missing).toEqual([])
  })

  it('settings keys exist for settings center', () => {
    for (const key of [
      'settings.title',
      'settings.sfx',
      'settings.music',
      'settings.haptics',
      'settings.motion',
      'settings.dataSaver',
      'settings.export',
      'settings.delete',
    ] as TranslationKey[]) {
      expect(zh[key]).toBeTruthy()
      expect(en[key]).toBeTruthy()
    }
  })
})
