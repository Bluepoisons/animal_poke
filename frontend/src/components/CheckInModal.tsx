import React, { useState, useCallback } from 'react'
import { useShop } from '../shop/useShop'
import { getCheckInRewardForDay } from '../shop/logic'
import { ITEM_DEFS, CHECK_IN_CYCLE_DAYS } from '../shop/constants'

interface CheckInModalProps {
  onClose: () => void
}

/** 签到弹窗组件：7 天日历 + 连签展示 + 断签提示 + 奖励预览 */
const CheckInModal: React.FC<CheckInModalProps> = ({ onClose }) => {
  const shop = useShop()
  const [toast, setToast] = useState<string | null>(null)

  const status = shop.getCheckInStatus()

  const handleCheckIn = useCallback(() => {
    const result = shop.checkIn()
    if (result.success) {
      let msg = `签到成功！+${result.reward} 🪙 +${result.rewardExp} XP`
      if (result.rewardItem) {
        msg += ` +${ITEM_DEFS[result.rewardItem].name} ×1 🎁`
      }
      if (result.wasReset) {
        msg = `断签重置，重新从第 1 天开始\n${msg}`
      }
      setToast(msg)
      setTimeout(() => setToast(null), 3000)
    } else {
      setToast('今日已签到')
      setTimeout(() => setToast(null), 2000)
    }
  }, [shop])

  return (
    <div style={styles.overlay} onClick={onClose}>
      <div style={styles.modal} onClick={e => e.stopPropagation()}>
        {/* 标题栏 */}
        <div style={styles.header}>
          <span style={styles.title}>📅 每日签到</span>
          <button onClick={onClose} style={styles.closeBtn}>×</button>
        </div>

        {/* 连签展示 */}
        <div style={styles.streakBox}>
          <div style={styles.streakMain}>
            连续签到 {status.hasCheckedInToday ? status.currentStreak : status.nextStreak} 天 🔥
          </div>
          <div style={styles.streakSub}>
            历史最高 {status.maxStreak} 天 · 累计签到 {status.totalCheckIns} 次
          </div>
        </div>

        {/* 断签提示 */}
        {status.isStreakBroken && (
          <div style={styles.breakNotice}>
            ⚠️ 上次签到已中断，今日签到将重新从第 1 天开始
          </div>
        )}

        {/* 7 天日历网格 */}
        <div style={styles.calendarGrid}>
          {Array.from({ length: CHECK_IN_CYCLE_DAYS }, (_, i) => {
            const day = i + 1
            const reward = getCheckInRewardForDay(day)
            const isDone = status.completedDays.includes(day)
            const isToday = day === status.todayCycleDay && !status.hasCheckedInToday
            const isDay7 = day === CHECK_IN_CYCLE_DAYS

            return (
              <div
                key={day}
                style={{
                  ...styles.calendarCell,
                  ...(isDone ? styles.cellDone : {}),
                  ...(isToday ? styles.cellToday : {}),
                  ...(isDay7 ? styles.cellDay7 : {}),
                }}
              >
                <span style={styles.cellDay}>D{day}</span>
                <span style={styles.cellGold}>{reward.gold}🪙</span>
                <span style={styles.cellExp}>+{reward.exp}XP</span>
                {isDay7 && <span style={styles.cellBonus}>🎁</span>}
                {isDone && <span style={styles.cellCheck}>✓</span>}
              </div>
            )
          })}
        </div>

        {/* 奖励预览 */}
        <div style={styles.rewardPreview}>
          <div style={styles.rewardRow}>
            <span style={styles.rewardLabel}>今日奖励：</span>
            <span style={styles.rewardValue}>{status.todayReward.gold} 🪙 + {status.todayReward.exp} XP</span>
            {status.todayReward.bonusItem && (
              <span style={styles.rewardBonus}>+ {ITEM_DEFS[status.todayReward.bonusItem].name} 🎁</span>
            )}
          </div>
          <div style={styles.rewardRow}>
            <span style={styles.rewardLabel}>明日奖励：</span>
            <span style={styles.rewardValue}>{status.tomorrowReward.gold} 🪙 + {status.tomorrowReward.exp} XP</span>
            {status.tomorrowReward.bonusItem && (
              <span style={styles.rewardBonus}>+ {ITEM_DEFS[status.tomorrowReward.bonusItem].name} 🎁</span>
            )}
          </div>
        </div>

        {/* 签到按钮 */}
        <button
          className="btn btn-primary"
          style={{
            ...styles.checkInBtn,
            ...(status.hasCheckedInToday ? styles.disabledBtn : {}),
          }}
          disabled={status.hasCheckedInToday}
          onClick={handleCheckIn}
        >
          {status.hasCheckedInToday ? '✓ 今日已签到' : '📅 立即签到'}
        </button>

        {/* Toast */}
        {toast && (
          <div style={styles.toast}>{toast}</div>
        )}
      </div>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  overlay: {
    position: 'fixed',
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    background: 'rgba(0,0,0,0.5)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    zIndex: 100,
  },
  modal: {
    background: 'var(--white)',
    borderRadius: 20,
    padding: 20,
    width: '90%',
    maxWidth: 360,
    maxHeight: '90vh',
    overflow: 'auto',
    position: 'relative',
    boxShadow: '0 8px 32px rgba(0,0,0,0.2)',
  },
  header: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: 16,
  },
  title: {
    fontSize: 18,
    fontWeight: 700,
    color: 'var(--ink)',
  },
  closeBtn: {
    background: 'none',
    border: 'none',
    fontSize: 24,
    color: 'var(--ink-3)',
    cursor: 'pointer',
    padding: '0 4px',
    lineHeight: 1,
  },
  streakBox: {
    textAlign: 'center',
    marginBottom: 12,
  },
  streakMain: {
    fontSize: 18,
    fontWeight: 700,
    color: 'var(--orange-dark)',
  },
  streakSub: {
    fontSize: 11,
    color: 'var(--ink-3)',
    marginTop: 4,
  },
  breakNotice: {
    background: 'rgba(255, 107, 107, 0.1)',
    borderRadius: 8,
    padding: '8px 12px',
    marginBottom: 12,
    fontSize: 12,
    color: 'var(--warn)',
    textAlign: 'center',
  },
  calendarGrid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(4, 1fr)',
    gap: 6,
    marginBottom: 16,
  },
  calendarCell: {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    justifyContent: 'center',
    padding: '8px 2px',
    borderRadius: 10,
    background: 'var(--cream-dark)',
    position: 'relative',
    minHeight: 56,
  },
  cellDone: {
    background: 'var(--success)',
    opacity: 0.85,
  },
  cellToday: {
    background: 'var(--orange-100)',
    border: '2px solid var(--orange)',
  },
  cellDay7: {
    border: '2px solid var(--coin)',
  },
  cellDay: {
    fontSize: 10,
    fontWeight: 700,
    color: 'var(--ink-2)',
  },
  cellGold: {
    fontSize: 11,
    fontWeight: 600,
    color: 'var(--ink)',
  },
  cellExp: {
    fontSize: 9,
    color: 'var(--ink-3)',
  },
  cellBonus: {
    fontSize: 12,
    position: 'absolute',
    top: 2,
    right: 4,
  },
  cellCheck: {
    position: 'absolute',
    top: 2,
    left: 4,
    fontSize: 12,
    color: 'var(--white)',
    fontWeight: 700,
  },
  rewardPreview: {
    background: 'var(--cream)',
    borderRadius: 10,
    padding: 12,
    marginBottom: 16,
    display: 'flex',
    flexDirection: 'column',
    gap: 6,
  },
  rewardRow: {
    display: 'flex',
    alignItems: 'center',
    gap: 6,
    fontSize: 13,
  },
  rewardLabel: {
    color: 'var(--ink-2)',
    fontWeight: 600,
    minWidth: 60,
  },
  rewardValue: {
    color: 'var(--ink)',
    fontWeight: 600,
  },
  rewardBonus: {
    color: 'var(--orange-dark)',
    fontWeight: 600,
    fontSize: 12,
  },
  checkInBtn: {
    width: '100%',
    padding: '12px 0',
    fontSize: 15,
    borderRadius: 12,
  },
  disabledBtn: {
    background: 'var(--ink-3)',
    color: 'var(--white)',
    boxShadow: 'none',
    cursor: 'not-allowed',
    opacity: 0.7,
  },
  toast: {
    position: 'absolute',
    bottom: 20,
    left: '50%',
    transform: 'translateX(-50%)',
    padding: '10px 20px',
    borderRadius: 10,
    fontSize: 13,
    fontWeight: 600,
    color: 'var(--white)',
    background: 'var(--success)',
    zIndex: 10,
    boxShadow: '0 4px 12px rgba(0,0,0,0.15)',
    whiteSpace: 'pre-wrap',
    textAlign: 'center',
    maxWidth: '90%',
  },
}

export default CheckInModal
