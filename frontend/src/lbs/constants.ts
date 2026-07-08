/** LBS 常量配置 */

/** 发现点刷新间隔（5 分钟） */
export const REFRESH_INTERVAL_MS = 5 * 60 * 1000

/** 发现点过期时间（10 分钟） */
export const DISCOVERY_TTL_MS = 10 * 60 * 1000

/** 发现点生成范围（500 米） */
export const DISCOVERY_RANGE_M = 500

/** 可捕获距离（50 米） */
export const CAPTURE_RANGE_M = 50

/** 定位请求配置 */
export const GEOLOCATION_OPTIONS: PositionOptions = {
  enableHighAccuracy: true,
  timeout: 10_000,
  maximumAge: 30_000,
}

/** localStorage 缓存 key */
export const LBS_CACHE_KEY = 'animal_poke_lbs_cache'

/** 稀有度掉率表 */
export const RARITY_SPAWN_RATES: Record<string, number> = {
  common: 0.60,
  uncommon: 0.25,
  rare: 0.10,
  epic: 0.04,
  legendary: 0.01,
}
