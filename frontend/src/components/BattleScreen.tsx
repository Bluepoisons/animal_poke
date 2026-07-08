import React from 'react'
import { useBattle } from '../battle/useBattle'
import { useStamina } from '../stamina/useStamina'
import { BATTLE_STAMINA_COST } from '../battle/constants'
import BattlePetSelect from './BattlePetSelect'
import BattleArena from './BattleArena'
import BattleResult from './BattleResult'

/** 匹配动画组件 */
const MatchingAnimation: React.FC = () => (
  <div style={{
    height: '100%',
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    justifyContent: 'center',
    gap: 16,
  }}>
    <div style={{ fontSize: 48 }}>⚔️</div>
    <div style={{
      fontSize: 16,
      fontWeight: 700,
      color: 'var(--ink)',
    }}>
      匹配中...
    </div>
    <div style={{
      width: 120,
      height: 4,
      borderRadius: 2,
      background: 'var(--orange-100)',
      overflow: 'hidden',
    }}>
      <div style={{
        width: '40%',
        height: '100%',
        borderRadius: 2,
        background: 'var(--orange)',
        animation: 'matchSlide 1.5s ease-in-out infinite',
      }} />
    </div>
    <style>{`
      @keyframes matchSlide {
        0% { transform: translateX(-120%); }
        50% { transform: translateX(150%); }
        100% { transform: translateX(-120%); }
      }
    `}</style>
  </div>
)

/** 战斗主界面：根据 phase 渲染不同子组件 */
const BattleScreen: React.FC = () => {
  const { state, enterSelect } = useBattle()
  const stamina = useStamina()
  const { phase } = state

  // 渲染对应阶段的子组件
  switch (phase) {
    case 'idle':
      return (
        <div style={{
          height: '100%',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          gap: 20,
          padding: 24,
        }}>
          <div style={{ fontSize: 56 }}>⚔️</div>
          <h2 style={{ fontSize: 20, fontWeight: 700, color: 'var(--ink)' }}>
            动物对战
          </h2>
          <div style={{ fontSize: 13, color: 'var(--ink-3)', textAlign: 'center', maxWidth: 260 }}>
            选择你的宠物，挑战野外对手！
            元素克制、策略切换、必杀技释放...
          </div>
          <div style={{ fontSize: 12, color: 'var(--ink-3)' }}>
            体力：{stamina.state.currentStamina} / {stamina.maxStamina}（消耗 {BATTLE_STAMINA_COST}）
          </div>
          <button
            className="btn btn-primary"
            style={{ fontSize: 16, padding: '12px 32px' }}
            onClick={enterSelect}
          >
            开始战斗
          </button>
        </div>
      )

    case 'selecting':
      return <BattlePetSelect />

    case 'matching':
      return <MatchingAnimation />

    case 'battling':
      return <BattleArena />

    case 'result':
      return <BattleResult />

    default:
      return null
  }
}

export default React.memo(BattleScreen)
