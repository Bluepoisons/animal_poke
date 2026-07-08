import React from 'react'
import type { LevelUpResult } from '../stamina/types'
import { getMaxStamina } from '../stamina/logic'
import { LEVEL_TABLE } from '../stamina/constants'

interface LevelUpToastProps {
  result: LevelUpResult
  onConfirm: () => void
}

const LevelUpToast: React.FC<LevelUpToastProps> = ({ result, onConfirm }) => {
  if (!result.leveledUp) return null

  const newMaxStamina = getMaxStamina(result.newLevel)
  const prevLevel = result.newLevel - result.crossedLevels.length
  const prevMaxStamina = getMaxStamina(prevLevel)
  const levelEntry = LEVEL_TABLE[result.newLevel - 1]

  return (
    <div style={styles.overlay} onClick={onConfirm}>
      <div style={styles.modal} onClick={(e) => e.stopPropagation()}>
        <div style={styles.title}>⬆️ 等级提升！</div>
        <div style={styles.levelNumber}>Lv.{result.newLevel}</div>
        <div style={styles.rewardList}>
          <div style={styles.rewardItem}>💰 +{result.rewardGold} 金币</div>
          <div style={styles.rewardItem}>⚡ 体力已恢复满</div>
          <div style={styles.rewardItem}>
            📈 体力上限 {prevMaxStamina} → {newMaxStamina}
          </div>
          {levelEntry && levelEntry.shopBonus > 0 && (
            <div style={styles.rewardItem}>
              🛒 商店稀有道具率 +{levelEntry.shopBonus}%
            </div>
          )}
        </div>
        <button style={styles.confirmButton} onClick={onConfirm}>
          确认
        </button>
      </div>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  overlay: {
    position: 'absolute',
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    background: 'rgba(0,0,0,0.6)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    zIndex: 100,
  },
  modal: {
    background: 'var(--white)',
    borderRadius: 20,
    padding: '32px 28px',
    textAlign: 'center',
    maxWidth: 280,
    width: '80%',
    boxShadow: '0 8px 32px rgba(0,0,0,0.3)',
    animation: 'popIn 0.3s ease',
  },
  title: {
    fontSize: 18,
    fontWeight: 700,
    color: 'var(--orange-dark)',
    marginBottom: 12,
  },
  levelNumber: {
    fontSize: 48,
    fontWeight: 800,
    color: 'var(--orange)',
    marginBottom: 16,
    textShadow: '0 2px 8px rgba(255,165,0,0.3)',
  },
  rewardList: {
    display: 'flex',
    flexDirection: 'column',
    gap: 8,
    marginBottom: 20,
  },
  rewardItem: {
    fontSize: 14,
    color: 'var(--ink-2)',
    fontWeight: 500,
  },
  confirmButton: {
    padding: '10px 32px',
    borderRadius: 20,
    border: 'none',
    background: 'var(--orange)',
    color: 'var(--white)',
    fontSize: 15,
    fontWeight: 700,
    cursor: 'pointer',
    fontFamily: 'inherit',
    boxShadow: '0 4px 0 var(--orange-dark)',
  },
}

export default LevelUpToast
