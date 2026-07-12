import type { Locale } from '../i18n'

/** Full client preferences (AP-053 settings center) */
export interface UserSettings {
  locale: Locale
  sfxEnabled: boolean
  musicEnabled: boolean
  hapticsEnabled: boolean
  reduceMotion: boolean
  dataSaver: boolean
  /** Non-sensitive prefs may sync after account bind */
  syncNonSensitive: boolean
  /** AP-109: prefer no-camera / no-location / low-mobility routes */
  homeMode: boolean
}

export const DEFAULT_USER_SETTINGS: UserSettings = {
  locale: 'zh',
  sfxEnabled: true,
  musicEnabled: true,
  hapticsEnabled: true,
  reduceMotion: false,
  dataSaver: false,
  syncNonSensitive: true,
  homeMode: false,
}

export const SETTINGS_STORAGE_KEY = 'animal_poke_user_settings'

/** Keys safe to sync after account bind (no precise location / photos / tokens) */
export const SYNCABLE_SETTING_KEYS: (keyof UserSettings)[] = [
  'locale',
  'sfxEnabled',
  'musicEnabled',
  'hapticsEnabled',
  'reduceMotion',
  'dataSaver',
  'homeMode',
]
