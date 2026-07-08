import React from 'react'
import { useBattle } from '../battle/useBattle'
import { MAX_ENERGY, STRATEGY_DEFS } from '../battle/constants'
import { RARITY_NAMES, RARITY_COLORS } from '../types'
import HealthBar from './HealthBar'
import BattleLog from './BattleLog'

/** 元素显示 */
const elementDisplay: Record<string, { emoji: string; color: string }> = {
  fire:   { emoji: '🔥', color: '#E55934' },
  water:  { emoji: '💧', color: '#4D8CFF' },
  grass:  { emoji: '🌿', color: '#4DCC4D' },
  light:  { emoji: '✨', color: '#FFCC33' },
  dark:   { emoji: '🌑', color: '#A65CF2' },
}

/** 战斗区域组件 */
const BattleArena: React.FC = () => {
  const { state, executeNextRound, useUltimate, setStrategy, useBattleItem, toggleAutoPlay } = useBattle()
  const { playerPet, enemyPet, round, maxRounds, log, strategy, isAutoPlaying } = state

  if (!playerPet || !enemyPet) return null

  const canUltimate = playerPet.energy >= MAX_ENERGY

  return (
    <div style={{ padding: '12px', height: '100%', display: 'flex', flexDirection: 'column', gap: 8 }}>
      {/* 回合数 */}
      <div style={{ textAlign: 'center', fontSize: 13, fontWeight: 700, color: 'var(--ink)' }}>
        回合 {round} / {maxRounds}
      </div>

      {/* 对手信息 */}
      <div style={{
        display: 'flex',
        alignItems: 'center',
        gap: 10,
        background: 'rgba(229,89,52,0.06)',
        borderRadius: 'var(--radius-md)',
        padding: '8px 12px',
      }}>
        <div style={{ fontSize: 28, textAlign: 'center', width: 40 }}>
          {enemyPet.emoji}
        </div>
        <div style={{ flex: 1 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
            <span style={{ fontWeight: 600, fontSize: 13, color: 'var(--ink)' }}>{enemyPet.name}</span>
            <span style={{ fontSize: 10, color: RARITY_COLORS[enemyPet.rarity], fontWeight: 600 }}>
              {RARITY_NAMES[enemyPet.rarity]}
            </span>
            <span style={{ fontSize: 11, color: elementDisplay[enemyPet.element]?.color }}>
              {elementDisplay[enemyPet.element]?.emoji} {enemyPet.element}
            </span>
          </div>
          <HealthBar
            current={enemyPet.currentHp}
            max={enemyPet.stats.hp}
            energy={enemyPet.energy}
            maxEnergy={MAX_ENERGY}
          />
        </div>
      </div>

      {/* 战斗场中间区域 */}
      <div style={{
        flex: 0,
        textAlign: 'center',
        fontSize: 40,
        padding: '4px 0',
      }}>
        <span style={{ margin: '0 20px' }}>{playerPet.emoji}</span>
        <span style={{ fontSize: 20, color: 'var(--ink-3)' }}>⚔️</span>
        <span style={{ margin: '0 20px' }}>{enemyPet.emoji}</span>
      </div>

      {/* 玩家信息 */}
      <div style={{
        display: 'flex',
        alignItems: 'center',
        gap: 10,
        background: 'rgba(255,140,66,0.06)',
        borderRadius: 'var(--radius-md)',
        padding: '8px 12px',
      }}>
        <div style={{ fontSize: 28, textAlign: 'center', width: 40 }}>
          {playerPet.emoji}
        </div>
        <div style={{ flex: 1 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
            <span style={{ fontWeight: 600, fontSize: 13, color: 'var(--ink)' }}>{playerPet.name}</span>
            <span style={{ fontSize: 10, color: RARITY_COLORS[playerPet.rarity], fontWeight: 600 }}>
              {RARITY_NAMES[playerPet.rarity]}
            </span>
            <span style={{ fontSize: 11, color: elementDisplay[playerPet.element]?.color }}>
              {elementDisplay[playerPet.element]?.emoji} {playerPet.element}
            </span>
          </div>
          <HealthBar
            current={playerPet.currentHp}
            max={playerPet.stats.hp}
            energy={playerPet.energy}
            maxEnergy={MAX_ENERGY}
            label="我方"
          />
        </div>
      </div>

      {/* 战斗日志 */}
      <BattleLog logs={log} />

      {/* 操作按钮栏 */}
      <div style={{ display: 'flex', gap: 6, justifyContent: 'space-between' }}>
        {/* 必杀按钮 */}
        <button
          className="btn"
          style={{
            flex: 1,
            fontSize: 12,
            padding: '8px 0',
            background: canUltimate ? '#A65CF2' : 'var(--ink-3)',
            color: canUltimate ? 'var(--white)' : 'rgba(255,255,255,0.4)',
            boxShadow: canUltimate ? '0 4px 0 #7C3AED' : 'none',
            cursor: canUltimate ? 'pointer' : 'not-allowed',
          }}
          onClick={canUltimate ? () => useUltimate() : undefined}
          disabled={!canUltimate}
        >
          {canUltimate ? '⚡ 必杀' : '必杀'}
        </button>

        {/* 策略切换 */}
        <button
          className="btn btn-primary"
          style={{ flex: 1, fontSize: 12, padding: '8px 0' }}
          onClick={() => {
            // 循环切换策略
            const strategies: Array<'aggressive' | 'balanced' | 'defensive'> = ['aggressive', 'balanced', 'defensive']
            const currentIdx = strategies.indexOf(strategy)
            setStrategy(strategies[(currentIdx + 1) % 3])
          }}
        >
          📋 {STRATEGY_DEFS[strategy].label}
        </button>

        {/* 道具 */}
        <button
          className="btn btn-primary"
          style={{ flex: 1, fontSize: 12, padding: '8px 0' }}
          onClick={() => useBattleItem('food_pack')}
        >
          🥫 回血
        </button>

        {/* 自动/暂停 */}
        <button
          className="btn btn-primary"
          style={{ flex: 1, fontSize: 12, padding: '8px 0' }}
          onClick={toggleAutoPlay}
        >
          {isAutoPlaying ? '⏸ 暂停' : '▶ 自动'}
        </button>

        {/* 手动下一回合（暂停时可用） */}
        {!isAutoPlaying && (
          <button
            className="btn btn-primary"
            style={{ flex: 1, fontSize: 12, padding: '8px 0' }}
            onClick={executeNextRound}
          >
            ▶ 下一回合
          </button>
        )}
      </div>
    </div>
  )
}

export default BattleArena
