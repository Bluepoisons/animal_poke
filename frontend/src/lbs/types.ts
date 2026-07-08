import type { SpeciesType, RarityTier } from '../types'

/** 地理位置（纬度 + 经度 + 精度） */
export interface GeoLocation {
  lat: number
  lng: number
  /** 定位精度（米），可选 */
  accuracy?: number
}

/** 发现点：玩家附近可探索的动物出现点 */
export interface DiscoveryPoint {
  /** 唯一 ID */
  id: string
  /** 纬度 */
  lat: number
  /** 经度 */
  lng: number
  /** 物种 */
  species: SpeciesType
  /** 稀有度 */
  rarity: RarityTier
  /** 生成时间戳（Unix ms） */
  spawnedAt: number
  /** 过期时间戳（Unix ms） */
  expiresAt: number
  /** 状态：可发现 / 已进入捕获范围 / 已过期 */
  status: 'available' | 'in_range' | 'expired'
}

/** LBS 系统完整状态 */
export interface LbsState {
  /** 定位状态 */
  geoStatus: 'idle' | 'locating' | 'located' | 'denied' | 'unsupported' | 'timeout'
  /** 玩家位置（null = 未获取） */
  playerLocation: GeoLocation | null
  /** 城市名 */
  cityName: string
  /** 省份名 */
  provinceName: string
  /** 发现点列表 */
  discoveryPoints: DiscoveryPoint[]
  /** 上次刷新时间戳 */
  lastRefreshTime: number
  /** 错误信息 */
  errorMsg: string
}

/** Reducer Action 类型 */
export type LbsAction =
  | { type: 'GEO_START' }
  | { type: 'GEO_SUCCESS'; location: GeoLocation }
  | { type: 'GEO_FAIL'; status: 'denied' | 'unsupported' | 'timeout'; errorMsg: string }
  | { type: 'GEO_CITY_SUCCESS'; city: string; province: string }
  | { type: 'GEO_CITY_FAIL' }
  | { type: 'REFRESH_POINTS'; points: DiscoveryPoint[]; now: number }
  | { type: 'REMOVE_POINT'; id: string }
  | { type: 'CLEAR_EXPIRED'; now: number }
  | { type: 'LOAD_STATE'; state: Partial<LbsState> }

/** LbsContext 暴露给组件的接口 */
export interface LbsContextValue {
  state: LbsState
  /** 请求定位 */
  requestLocation: () => void
  /** 手动刷新发现点 */
  refreshPoints: () => void
  /** 移除发现点（捕获成功后调用） */
  removePoint: (id: string) => void
  /** 获取玩家附近的 in_range 发现点 */
  getInRangePoints: () => DiscoveryPoint[]
  /** 距下次刷新的秒数 */
  nextRefreshIn: number
}
