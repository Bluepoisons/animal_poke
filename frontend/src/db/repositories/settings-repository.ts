import { getDB } from '../db'
import type { AppSettings } from '../types'

/** 默认设置 */
const DEFAULT_SETTINGS: AppSettings = {
  key: 'prefs',
  soundEnabled: true,
  musicEnabled: true,
  privacyConsented: false,
}

/** 应用设置数据访问层 */
export const SettingsRepository = {
  /** 获取设置（不存在则返回默认值） */
  async get(): Promise<AppSettings> {
    const db = await getDB()
    const result = await db.get('settings', 'prefs')
    return result ?? DEFAULT_SETTINGS
  },

  /** 保存设置 */
  async save(settings: AppSettings): Promise<void> {
    const db = await getDB()
    await db.put('settings', settings)
  },
}
