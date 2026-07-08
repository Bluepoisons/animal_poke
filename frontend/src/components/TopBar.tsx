import React from 'react'
import { useStamina } from '../stamina/useStamina'

interface TopBarProps {
  location: string
  weather: string
  // 以下字段改为可选，优先从 Context 读取
  level?: number
  stamina?: number
  maxStamina?: number
  gold?: number
}

const TopBar: React.FC<TopBarProps> = ({ location, weather, ...props }) => {
  const staminaState = useStamina()

  const level = props.level ?? staminaState.state.level
  const currentStamina = props.stamina ?? staminaState.state.currentStamina
  const maxStamina = props.maxStamina ?? staminaState.maxStamina
  const gold = props.gold ?? staminaState.state.gold

  return (
    <div style={styles.container}>
      <div style={styles.left}>
        <div style={styles.avatar}>🐱</div>
        <span className="pill">Lv.{level}</span>
        <span className="pill">⚡ {currentStamina}/{maxStamina}</span>
        <span className="pill">🪙 {gold}</span>
      </div>
      <span className="pill">{weather} {location}</span>
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
