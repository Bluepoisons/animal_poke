import React from 'react'
import { useStamina } from '../stamina/useStamina'
import { useLbs } from '../lbs/useLbs'
import { useWeather } from '../weather/useWeather'
import { useEconomy } from '../economy/useEconomy'

interface TopBarProps {
  location?: string
  weather?: string
  // 以下字段改为可选，优先从 Context 读取
  level?: number
  stamina?: number
  maxStamina?: number
  gold?: number
}

const TopBar: React.FC<TopBarProps> = ({ location, weather, ...props }) => {
  const staminaState = useStamina()
  let lbsState: { cityName: string; provinceName: string } | null = null
  try {
    const lbs = useLbs()
    lbsState = lbs.state
  } catch {
    // LbsProvider 未挂载时忽略
  }
  let weatherState: ReturnType<typeof useWeather> | null = null
  try {
    weatherState = useWeather()
  } catch {
    // WeatherProvider 未挂载时忽略
  }
  let todayNetFlow = 0
  try {
    const economy = useEconomy()
    const stats = economy.getStats(staminaState.state.gold)
    todayNetFlow = stats.todayNetFlow
  } catch {
    // EconomyProvider 未挂载时忽略
  }

  const level = props.level ?? staminaState.state.level
  const currentStamina = props.stamina ?? staminaState.state.currentStamina
  const maxStamina = props.maxStamina ?? staminaState.maxStamina
  const gold = props.gold ?? staminaState.state.gold

  // 城市名：优先从 props，其次从 LbsContext，最后默认"未知"
  const displayLocation = location ?? (lbsState?.cityName || '未知')

  // 天气：优先从 props，其次从 WeatherContext，最后默认
  const weatherEmoji = weather ?? weatherState?.todayMeta?.emoji ?? '☀️'
  const weatherName = weatherState?.todayMeta?.name ?? ''
  const displayWeather = weatherName
    ? `${weatherEmoji} ${weatherName} · ${displayLocation}`
    : `${weatherEmoji} ${displayLocation}`

  return (
    <div style={styles.container}>
      <div style={styles.left}>
        <div style={styles.avatar}>🐱</div>
        <span className="pill">Lv.{level}</span>
        <span className="pill">⚡ {currentStamina}/{maxStamina}</span>
        <span className="pill" style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', lineHeight: 1.1 }}>
          <span>🪙 {gold}</span>
          {todayNetFlow !== 0 && (
            <span style={{
              fontSize: 9,
              color: todayNetFlow > 0 ? 'var(--success)' : 'var(--warn)',
            }}>
              {todayNetFlow > 0 ? '+' : ''}{todayNetFlow} 今日
            </span>
          )}
        </span>
      </div>
      <span className="pill">{displayWeather}</span>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    height: 56,
    background: 'var(--orange-dark)',
    borderRadius: '0 0 16px 16px',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: '0 12px',
    color: 'var(--white)',
    flexShrink: 0,
    zIndex: 2,
  },
  left: {
    display: 'flex',
    alignItems: 'center',
    gap: 6,
  },
  avatar: {
    width: 32,
    height: 32,
    borderRadius: '50%',
    background: 'var(--white)',
    color: 'var(--orange-dark)',
    display: 'grid',
    placeItems: 'center',
    fontSize: 16,
  },
}

export default TopBar
