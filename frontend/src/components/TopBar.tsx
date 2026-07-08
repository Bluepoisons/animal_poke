import React from 'react'

interface TopBarProps {
  level: number
  stamina: number
  maxStamina: number
  gold: number
  location: string
  weather: string
}

const TopBar: React.FC<TopBarProps> = ({ level, stamina, maxStamina, gold, location, weather }) => {
  return (
    <div style={styles.container}>
      <div style={styles.left}>
        <div style={styles.avatar}>🐱</div>
        <span className="pill">Lv.{level}</span>
        <span className="pill">⚡ {stamina}/{maxStamina}</span>
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
