import React from 'react'

interface HealthBarProps {
  current: number
  max: number
  energy: number
  maxEnergy: number
  label?: string
  showEnergy?: boolean
}

/** 血条 + 能量条组件 */
const HealthBar: React.FC<HealthBarProps> = ({ current, max, energy, maxEnergy, label, showEnergy = true }) => {
  const hpPercent = Math.max(0, Math.min(100, (current / max) * 100))
  const energyPercent = Math.max(0, Math.min(100, (energy / maxEnergy) * 100))

  // 颜色：>50% 绿，25~50% 黄，<25% 红
  const hpColor = hpPercent > 50 ? 'var(--success)' : hpPercent > 25 ? 'var(--stamina)' : 'var(--warn)'
  const isEnergyFull = energy >= maxEnergy

  return (
    <div style={{ width: '100%' }}>
      {label && (
        <div style={{ fontSize: 11, fontWeight: 600, color: 'var(--ink-2)', marginBottom: 2 }}>{label}</div>
      )}
      {/* 血条 */}
      <div style={{
        width: '100%',
        height: 12,
        borderRadius: 6,
        background: 'rgba(0,0,0,0.12)',
        overflow: 'hidden',
        position: 'relative',
      }}>
        <div style={{
          width: `${hpPercent}%`,
          height: '100%',
          borderRadius: 6,
          background: hpColor,
          transition: 'width 300ms ease-out, background 300ms ease-out',
        }} />
      </div>
      {/* HP 数值 */}
      <div style={{ fontSize: 10, fontWeight: 600, color: 'var(--ink-3)', marginTop: 1 }}>
        {current} / {max}
      </div>
      {/* 能量条 */}
      {showEnergy && (
        <div style={{
          width: '100%',
          height: 6,
          borderRadius: 3,
          background: 'rgba(0,0,0,0.08)',
          overflow: 'hidden',
          marginTop: 2,
        }}>
          <div style={{
            width: `${energyPercent}%`,
            height: '100%',
            borderRadius: 3,
            background: isEnergyFull
              ? 'linear-gradient(90deg, #6366f1, #a855f7)'
              : 'linear-gradient(90deg, #60a5fa, #818cf8)',
            transition: 'width 300ms ease-out',
            ...(isEnergyFull ? {
              animation: 'pulse 1.5s ease-in-out infinite',
            } : {}),
          }} />
        </div>
      )}
    </div>
  )
}

export default HealthBar
