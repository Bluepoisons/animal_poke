import React, { useState, useCallback, useMemo } from 'react'
import { useDispatch } from '../economy/useDispatch'
import { useStamina } from '../stamina/useStamina'
import { useAnimalStore } from '../hooks/useAnimalStore'
import { RARITY_NAMES, RARITY_COLORS } from '../types'
import type { DispatchMissionType } from '../economy/types'
import type { AnimalRecord } from '../db/types'

/** 格式化倒计时为 mm:ss */
function formatCountdown(seconds: number): string {
  const m = Math.floor(seconds / 60)
  const s = seconds % 60
  return `${m}:${String(s).padStart(2, '0')}`
}

const DispatchScreen: React.FC = () => {
  const dispatchCtx = useDispatch()
  const stamina = useStamina()
  const { animals } = useAnimalStore()
  const [selectedMissionType, setSelectedMissionType] = useState<DispatchMissionType | null>(null)
  const [toast, setToast] = useState<{ msg: string; type: 'success' | 'fail' } | null>(null)

  const showToast = useCallback((msg: string, type: 'success' | 'fail' = 'success') => {
    setToast({ msg, type })
    setTimeout(() => setToast(null), 2000)
  }, [])

  // 活跃任务（含已完成未领取）
  const activeMissions = useMemo(
    () => dispatchCtx.state.missions.filter(m => m.status !== 'collected'),
    [dispatchCtx.state.missions]
  )

  // 可派遣宠物（已解锁且未在派遣中）
  const availablePets = useMemo(
    () => animals.filter(a => a.unlocked && !dispatchCtx.getPetMission(a.id)),
    [animals, dispatchCtx]
  )

  const handleStart = useCallback((missionType: DispatchMissionType, pet: AnimalRecord) => {
    const result = dispatchCtx.startMission(missionType, pet.id, pet.rarity)
    if (result.success) {
      showToast(`派遣成功！${pet.no}`, 'success')
      setSelectedMissionType(null)
    } else {
      const reasonMap: Record<string, string> = {
        no_slots: '没有可用槽位',
        pet_busy: '宠物正在派遣中',
        insufficient_stamina: '体力不足',
        pet_not_found: '宠物不存在',
      }
      showToast(reasonMap[result.reason ?? ''] ?? '派遣失败', 'fail')
    }
  }, [dispatchCtx, showToast])

  const handleCollect = useCallback((missionId: string) => {
    const result = dispatchCtx.collectMission(missionId)
    if (result.success && result.rewards) {
      let msg = `领取成功！+${result.rewards.gold} 🪙`
      if (result.rewards.droppedItem) {
        msg += ` +道具`
      }
      showToast(msg, 'success')
    } else {
      const reasonMap: Record<string, string> = {
        not_completed: '任务尚未完成',
        not_found: '任务不存在',
      }
      showToast(reasonMap[result.reason ?? ''] ?? '领取失败', 'fail')
    }
  }, [dispatchCtx, showToast])

  const handleSpeedUp = useCallback((missionId: string) => {
    const result = dispatchCtx.speedUpMission(missionId)
    if (result.success) {
      showToast('加速完成！-50 🪙', 'success')
    } else {
      const reasonMap: Record<string, string> = {
        insufficient_gold: '金币不足',
        not_found: '任务不存在',
        already_completed: '任务已完成',
      }
      showToast(reasonMap[result.reason ?? ''] ?? '加速失败', 'fail')
    }
  }, [dispatchCtx, showToast])

  return (
    <div style={styles.container}>
      {/* 状态栏 */}
      <div style={styles.statusBar}>
        <span style={styles.statusItem}>🗺️ 槽位: {dispatchCtx.availableSlots}</span>
        <span style={styles.statusItem}>⚡ {stamina.state.currentStamina}/{stamina.maxStamina}</span>
        <span style={styles.statusItem}>📊 今日: {dispatchCtx.state.todayDispatchCount}</span>
      </div>

      {/* 活跃任务 */}
      {activeMissions.length > 0 && (
        <div style={styles.section}>
          <div style={styles.sectionTitle}>进行中的派遣</div>
          {activeMissions.map(mission => {
            const def = dispatchCtx.missionDefs.find(d => d.type === mission.type)
            const countdown = dispatchCtx.getMissionCountdown(mission.id)
            const isCompleted = mission.status === 'completed'
            const pet = animals.find(a => a.id === mission.petId)
            return (
              <div key={mission.id} style={{
                ...styles.missionCard,
                borderColor: isCompleted ? 'var(--success)' : RARITY_COLORS[mission.petRarity],
              }}>
                <div style={styles.missionHeader}>
                  <span style={{ fontSize: 24 }}>{def?.icon}</span>
                  <div style={styles.missionInfo}>
                    <span style={styles.missionName}>{def?.name}</span>
                    <span style={styles.missionPet}>{pet?.no ?? mission.petId} · {RARITY_NAMES[mission.petRarity]}</span>
                  </div>
                  {isCompleted ? (
                    <span style={styles.completedBadge}>✓ 完成</span>
                  ) : (
                    <span style={styles.countdownBadge}>⏰ {formatCountdown(countdown)}</span>
                  )}
                </div>
                <div style={styles.rewardRow}>
                  <span>💰 {mission.rewards.gold}</span>
                  {mission.rewards.droppedItem && <span>🎁 道具</span>}
                  <span>❤️ +{mission.rewards.affinity}</span>
                </div>
                <div style={styles.missionActions}>
                  {isCompleted ? (
                    <button
                      className="btn btn-primary"
                      style={styles.actionBtn}
                      onClick={() => handleCollect(mission.id)}
                    >
                      领取奖励
                    </button>
                  ) : (
                    <button
                      className="btn"
                      style={{ ...styles.actionBtn, background: 'var(--coin)', color: 'var(--white)' }}
                      onClick={() => handleSpeedUp(mission.id)}
                      disabled={stamina.state.gold < 50}
                    >
                      加速 (50🪙)
                    </button>
                  )}
                </div>
              </div>
            )
          })}
        </div>
      )}

      {/* 任务类型选择 */}
      <div style={styles.section}>
        <div style={styles.sectionTitle}>派遣任务</div>
        {dispatchCtx.missionDefs.map(def => {
          const canDispatch = dispatchCtx.availableSlots > 0 && stamina.state.currentStamina >= def.staminaCost
          return (
            <div key={def.type} style={styles.missionTypeCard}>
              <div style={styles.missionTypeHeader}>
                <span style={{ fontSize: 28 }}>{def.icon}</span>
                <div style={styles.missionTypeInfo}>
                  <span style={styles.missionTypeName}>{def.name}</span>
                  <span style={styles.missionTypeDesc}>{def.description}</span>
                </div>
              </div>
              <div style={styles.missionTypeStats}>
                <span>⏱ {def.durationMin}分钟</span>
                <span>⚡ {def.staminaCost}体力</span>
                <span>💰 {def.baseGold}+金币</span>
              </div>
              <button
                className="btn btn-primary"
                style={{
                  ...styles.dispatchBtn,
                  ...(!canDispatch ? styles.disabledBtn : {}),
                }}
                disabled={!canDispatch}
                onClick={() => setSelectedMissionType(def.type)}
              >
                {dispatchCtx.availableSlots <= 0 ? '无可用槽位' : '选择宠物派遣'}
              </button>
            </div>
          )
        })}
      </div>

      {/* 宠物选择弹窗 */}
      {selectedMissionType && (
        <div style={styles.overlay} onClick={() => setSelectedMissionType(null)}>
          <div style={styles.petPicker} onClick={e => e.stopPropagation()}>
            <div style={styles.petPickerTitle}>选择宠物</div>
            {availablePets.length === 0 ? (
              <div style={styles.emptyText}>没有可派遣的宠物</div>
            ) : (
              <div style={styles.petList}>
                {availablePets.map(pet => (
                  <button
                    key={pet.id}
                    style={{
                      ...styles.petCard,
                      borderColor: RARITY_COLORS[pet.rarity],
                    }}
                    onClick={() => handleStart(selectedMissionType, pet)}
                  >
                    <span style={styles.petNo}>{pet.no}</span>
                    <span style={{ ...styles.petRarity, color: RARITY_COLORS[pet.rarity] }}>
                      {RARITY_NAMES[pet.rarity]}
                    </span>
                  </button>
                ))}
              </div>
            )}
            <button
              className="btn"
              style={styles.cancelBtn}
              onClick={() => setSelectedMissionType(null)}
            >
              取消
            </button>
          </div>
        </div>
      )}

      {/* Toast */}
      {toast && (
        <div style={{
          ...styles.toast,
          ...(toast.type === 'success' ? styles.toastSuccess : styles.toastFail),
        }}>
          {toast.msg}
        </div>
      )}
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    flex: 1,
    overflow: 'auto',
    background: 'var(--cream)',
    padding: 12,
    paddingBottom: 20,
    position: 'relative',
  },
  statusBar: {
    display: 'flex',
    justifyContent: 'space-around',
    background: 'var(--white)',
    borderRadius: 'var(--radius-lg)',
    padding: '10px 12px',
    marginBottom: 12,
    boxShadow: 'var(--shadow-card)',
  },
  statusItem: {
    fontSize: 13,
    fontWeight: 600,
    color: 'var(--ink)',
  },
  section: {
    marginBottom: 12,
  },
  sectionTitle: {
    fontSize: 15,
    fontWeight: 700,
    color: 'var(--ink)',
    marginBottom: 8,
  },
  missionCard: {
    background: 'var(--white)',
    borderRadius: 'var(--radius-lg)',
    padding: 12,
    marginBottom: 8,
    boxShadow: 'var(--shadow-card)',
    borderLeft: '4px solid var(--orange)',
  },
  missionHeader: {
    display: 'flex',
    alignItems: 'center',
    gap: 10,
    marginBottom: 8,
  },
  missionInfo: {
    flex: 1,
    display: 'flex',
    flexDirection: 'column',
  },
  missionName: {
    fontSize: 14,
    fontWeight: 700,
    color: 'var(--ink)',
  },
  missionPet: {
    fontSize: 11,
    color: 'var(--ink-3)',
  },
  completedBadge: {
    fontSize: 12,
    fontWeight: 700,
    color: 'var(--success)',
    background: 'var(--orange-50)',
    padding: '2px 10px',
    borderRadius: 12,
  },
  countdownBadge: {
    fontSize: 12,
    fontWeight: 600,
    color: 'var(--orange-dark)',
    background: 'var(--orange-50)',
    padding: '2px 10px',
    borderRadius: 12,
  },
  rewardRow: {
    display: 'flex',
    gap: 12,
    fontSize: 12,
    color: 'var(--ink-2)',
    marginBottom: 8,
  },
  missionActions: {
    display: 'flex',
    justifyContent: 'flex-end',
  },
  actionBtn: {
    padding: '6px 16px',
    fontSize: 12,
    borderRadius: 'var(--radius-md)',
  },
  missionTypeCard: {
    background: 'var(--white)',
    borderRadius: 'var(--radius-lg)',
    padding: 14,
    marginBottom: 8,
    boxShadow: 'var(--shadow-card)',
  },
  missionTypeHeader: {
    display: 'flex',
    alignItems: 'center',
    gap: 10,
    marginBottom: 8,
  },
  missionTypeInfo: {
    flex: 1,
    display: 'flex',
    flexDirection: 'column',
  },
  missionTypeName: {
    fontSize: 14,
    fontWeight: 700,
    color: 'var(--ink)',
  },
  missionTypeDesc: {
    fontSize: 11,
    color: 'var(--ink-3)',
  },
  missionTypeStats: {
    display: 'flex',
    gap: 12,
    fontSize: 12,
    color: 'var(--ink-2)',
    marginBottom: 10,
  },
  dispatchBtn: {
    width: '100%',
    padding: '8px 0',
    fontSize: 13,
    borderRadius: 'var(--radius-md)',
  },
  disabledBtn: {
    background: 'var(--ink-3)',
    color: 'var(--white)',
    boxShadow: 'none',
    cursor: 'not-allowed',
    opacity: 0.7,
  },
  overlay: {
    position: 'absolute',
    inset: 0,
    background: 'rgba(74,44,26,0.6)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    zIndex: 50,
    padding: 20,
  },
  petPicker: {
    width: '100%',
    maxWidth: 300,
    background: 'var(--white)',
    borderRadius: 20,
    padding: 14,
    display: 'flex',
    flexDirection: 'column',
    gap: 10,
  },
  petPickerTitle: {
    fontSize: 15,
    fontWeight: 700,
    color: 'var(--ink)',
    textAlign: 'center',
  },
  petList: {
    display: 'flex',
    flexDirection: 'column',
    gap: 6,
    maxHeight: 300,
    overflow: 'auto',
  },
  petCard: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    padding: '10px 14px',
    borderRadius: 'var(--radius-md)',
    border: '2px solid var(--orange)',
    background: 'var(--cream)',
    cursor: 'pointer',
    fontFamily: 'inherit',
    fontSize: 14,
    fontWeight: 600,
    color: 'var(--ink)',
  },
  petNo: {
    fontWeight: 700,
    color: 'var(--orange-dark)',
  },
  petRarity: {
    fontSize: 12,
    fontWeight: 600,
  },
  cancelBtn: {
    width: '100%',
    padding: '8px 0',
    fontSize: 13,
    borderRadius: 'var(--radius-md)',
    background: 'var(--cream-dark)',
    color: 'var(--ink-2)',
  },
  emptyText: {
    textAlign: 'center',
    color: 'var(--ink-3)',
    fontSize: 13,
    padding: 20,
  },
  toast: {
    position: 'absolute',
    bottom: 20,
    left: '50%',
    transform: 'translateX(-50%)',
    padding: '10px 20px',
    borderRadius: 'var(--radius-md)',
    fontSize: 13,
    fontWeight: 600,
    color: 'var(--white)',
    zIndex: 10,
    boxShadow: '0 4px 12px rgba(0,0,0,0.15)',
    whiteSpace: 'nowrap',
  },
  toastSuccess: {
    background: 'var(--success)',
  },
  toastFail: {
    background: 'var(--warn)',
  },
}

export default DispatchScreen
