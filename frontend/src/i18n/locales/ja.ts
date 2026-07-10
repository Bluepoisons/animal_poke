/** Japanese stub — falls back to English for missing keys at runtime */
import type { TranslationKey } from './zh'
import { en } from './en'

/** Stub locale: copy en until full JA translation lands */
export const ja: Partial<Record<TranslationKey, string>> = {
  ...en,
  'app.name': 'アニマルポケ',
  'settings.language': '言語',
  'settings.japanese': '日本語',
  'tab.settings': '設定',
  'common.back': '戻る',
}
