import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { I18nProvider, useI18n } from './index'
import { zh, type TranslationKey } from './locales/zh'
import { en } from './locales/en'

function wrapper({ children }: { children: React.ReactNode }) {
  return <I18nProvider>{children}</I18nProvider>
}

describe('i18n', () => {
  beforeEach(() => {
    localStorage.clear()
    // jsdom defaults navigator.language to 'en-US'; force zh for default tests
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

  it('handles missing key gracefully', () => {
    const { result } = renderHook(() => useI18n(), { wrapper })
    expect(result.current.t('nonexistent.key' as TranslationKey)).toBe('nonexistent.key')
  })

  it('substitutes params in template', () => {
    const { result } = renderHook(() => useI18n(), { wrapper })
    // Key without placeholders returns as-is even with params
    const text = result.current.t('collection.unlocked', { count: 5 })
    expect(text).toBe(zh['collection.unlocked'])
  })

  it('all zh keys have en translations', () => {
    const zhKeys = Object.keys(zh) as TranslationKey[]
    const enKeys = Object.keys(en)
    const missing = zhKeys.filter(k => !enKeys.includes(k))
    expect(missing).toEqual([])
  })
})
