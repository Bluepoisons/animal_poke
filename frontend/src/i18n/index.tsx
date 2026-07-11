import { createContext, useContext, useState, useCallback, useEffect, type ReactNode } from 'react'
import { zh, type TranslationKey } from './locales/zh'
import { en } from './locales/en'

export type Locale = 'zh' | 'en'

const dictionaries: Record<Locale, Record<TranslationKey, string>> = {
  zh,
  en,
}

const STORAGE_KEY = 'animal-poke-locale'

interface I18nContextValue {
  locale: Locale
  setLocale: (locale: Locale) => void
  t: (key: TranslationKey | string, params?: Record<string, string | number>) => string
  /** Production locales. Japanese is intentionally unavailable until complete. */
  supportedLocales: readonly Locale[]
}

const I18nContext = createContext<I18nContextValue | null>(null)

function detectInitialLocale(): Locale {
  if (typeof localStorage !== 'undefined') {
    try {
      const saved = localStorage.getItem(STORAGE_KEY)
      if (saved === 'zh' || saved === 'en') return saved
    } catch { /* ignore */ }
  }
  if (typeof navigator !== 'undefined') {
    const lang = navigator.language.toLowerCase()
    if (lang.startsWith('en')) return 'en'
  }
  return 'zh'
}

/**
 * Resolve message: current locale → zh fallback → key string.
 * Never throws on missing keys (AP-053).
 */
export function resolveMessage(
  locale: Locale,
  key: string,
  params?: Record<string, string | number>,
): string {
  const dict = dictionaries[locale] ?? zh
  const template =
    (dict as Record<string, string>)[key] ??
    (zh as Record<string, string>)[key] ??
    (en as Record<string, string>)[key] ??
    key
  if (!params) return template
  return template.replace(/\{(\w+)\}/g, (_, name: string) =>
    params[name] !== undefined ? String(params[name]) : `{${name}}`,
  )
}

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>(detectInitialLocale)

  const setLocale = useCallback((newLocale: Locale) => {
    setLocaleState(newLocale)
    try {
      localStorage.setItem(STORAGE_KEY, newLocale)
    } catch { /* ignore */ }
  }, [])

  useEffect(() => {
    document.documentElement.lang = locale === 'zh' ? 'zh-CN' : 'en'
  }, [locale])

  const t = useCallback(
    (key: TranslationKey | string, params?: Record<string, string | number>): string => {
      return resolveMessage(locale, key, params)
    },
    [locale],
  )

  return (
    <I18nContext.Provider value={{ locale, setLocale, t, supportedLocales: ['zh', 'en'] }}>
      {children}
    </I18nContext.Provider>
  )
}

export function useI18n(): I18nContextValue {
  const ctx = useContext(I18nContext)
  if (!ctx) {
    throw new Error('useI18n must be used within I18nProvider')
  }
  return ctx
}

export { TranslationKey }
