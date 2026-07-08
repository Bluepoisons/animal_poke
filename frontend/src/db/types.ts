import type { CardEntry } from '../types'

/** 动物收藏记录（继承 CardEntry，后续可扩展 DB 专属字段） */
export interface AnimalRecord extends CardEntry {}

/** 应用设置（单条记录，key 固定为 'prefs'） */
export interface AppSettings {
  key: 'prefs'
  soundEnabled: boolean
  musicEnabled: boolean
  privacyConsented: boolean
}
