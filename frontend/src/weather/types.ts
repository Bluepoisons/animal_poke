/** 天气类型 — 与 BattleContext 中 WeatherType 保持一致 */
export type WeatherType = 'sunny' | 'cloudy' | 'overcast' | 'rainy' | 'snowy' | 'foggy' | 'extreme'

/** 天气元数据（用于 UI 展示） */
export interface WeatherMeta {
  type: WeatherType
  /** 中文名 */
  name: string
  /** 显示 emoji */
  emoji: string
  /** 简短描述 */
  desc: string
  /** 健康风险描述（雨/雪显示感冒警告） */
  riskDesc?: string
}

/** 天气元数据表 */
export const WEATHER_META: Record<WeatherType, WeatherMeta> = {
  sunny:    { type: 'sunny',    name: '晴', emoji: '☀️', desc: '光线充足 · 捕获 +5%' },
  cloudy:   { type: 'cloudy',   name: '多云', emoji: '⛅', desc: '多云' },
  overcast: { type: 'overcast', name: '阴', emoji: '☁️', desc: '光线偏暗 · 捕获 -5%' },
  rainy:    { type: 'rainy',    name: '雨', emoji: '🌧️', desc: '视线受阻 · 捕获 -15%', riskDesc: '出行感冒概率 8%' },
  snowy:    { type: 'snowy',    name: '雪', emoji: '❄️', desc: '低温操作受阻 · 捕获 -10%', riskDesc: '出行感冒概率 6%' },
  foggy:    { type: 'foggy',    name: '雾', emoji: '🌫️', desc: '检测距离缩短 · 捕获 -10%' },
  extreme:  { type: 'extreme',  name: '极端天气', emoji: '⚠️', desc: '户外玩法暂停 · 注意安全' },
}

/** 单日天气 */
export interface CurrentWeather {
  /** 天气类型 */
  weather: WeatherType
  /** 日期标签（"周一"~"周日"） */
  dayLabel: string
}

/** 一周天气数组（周一~周日，7 天） */
export type WeekWeather = [
  CurrentWeather, CurrentWeather, CurrentWeather, CurrentWeather,
  CurrentWeather, CurrentWeather, CurrentWeather,
]

/** 天气状态 */
export interface WeatherState {
  /** 当前周的天气（7 天数组） */
  week: WeekWeather | null
  /** 本周开始时间戳（周日 0:00） */
  weekStart: number
  /** 所属地级市 */
  city: string
  /** 数据源：'internal' = 游戏内随机, 'backend' = 后端下发 */
  source: 'internal' | 'backend'
  /** 加载状态 */
  status: 'idle' | 'loading' | 'loaded' | 'error'
  /** 错误信息 */
  errorMsg: string
}

/** Reducer Action */
export type WeatherAction =
  | { type: 'SET_WEEK'; week: WeekWeather; weekStart: number; city: string; source: 'internal' | 'backend' }
  | { type: 'LOADING' }
  | { type: 'ERROR'; msg: string }
  | { type: 'RESET' }

/** 捕获修正结果 */
export interface CaptureModifierResult {
  modifier: number        // 百分比修正值（如 +5, -15）
  multiplier: number      // 倍率修正（如 1.05, 0.85）
  description: string     // 展示文本
}

/** 感冒判定结果 */
export interface ColdCheckResult {
  isRisky: boolean        // 当前天气存在感冒风险
  probability: number     // 感冒概率（0~1，如 0.08）
  description: string     // 展示文本
}

/** WeatherContext 暴露给组件的接口 */
export interface WeatherContextValue {
  state: WeatherState
  /** 今天的天气 */
  today: CurrentWeather | null
  /** 今日天气元数据 */
  todayMeta: WeatherMeta | null
  /** 获取今日捕获修正 */
  getCaptureModifier: () => CaptureModifierResult
  /** 获取今日感冒风险 */
  getColdRisk: () => ColdCheckResult
  /** 获取今日战斗属性修正 */
  getBattleModifier: () => WeatherType
  /** 手动刷新天气（跨城市切换时调用） */
  refreshWeather: (city: string) => void
}
