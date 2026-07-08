import React from 'react'
import { useBattle } from '../battle/useBattle'

/** 战斗结果组件 */
const BattleResult: React.FC = () => {
  const { state, finishBattle, reset } = useBattle()
  const { result, rewards } = state

  if (!result) return null

  const isWin = result === 'win'
  const isDraw = result === 'draw'

  // 结果标题和颜色
  const title = isWin ? '胜利！' : isDraw ? '平局' : '失败'
  const titleColor = isWin ? 'var(--success)' : isDraw ? 'var(--stamina)' : 'var(--warn)'
  const emoji = isWin ? '🎉' : isDraw ? '🤝' : '😢'

  return (
    <div style={{
      padding: 24,
      height: '100%',
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      gap: 16,
    }}>
      {/* 结果标题 */}
      <div style={{
        fontSize: 36,
        fontWeight: 800,
        color: titleColor,
        textAlign: 'center',
      }}>
        {emoji} {title}
      </div>

      {/* 奖励展示 */}
      {rewards && (
        <div style={{
          background: 'var(--white)',
          borderRadius: 'var(--radius-lg)',
          padding: 16,
          boxShadow: 'var(--shadow-card)',
          textAlign: 'center',
          width: '80%',
        }}>
          <div style={{ fontSize: 16, fontWeight: 700, color: 'var(--ink)', marginBottom: 8 }}>
            战斗奖励
          </div>
          <div style={{ fontSize: 20, fontWeight: 700, color: 'var(--coin)' }}>
            💰 {rewards.gold} 金币
          </div>
          {rewards.droppedItem && (
            <div style={{ fontSize: 13, color: 'var(--success)', marginTop: 6 }}>
              🎁 获得道具：{rewards.droppedItem}
            </div>
          )}
        </div>
      )}

      {/* 操作按钮 */}
      <div style={{ display: 'flex', gap: 12, width: '80%' }}>
        <button
          className="btn btn-primary"
          style={{ flex: 1, fontSize: 14, padding: '10px 0' }}
          onClick={finishBattle}
        >
          再战
        </button>
        <button
          className="btn"
          style={{
            flex: 1,
            fontSize: 14,
            padding: '10px 0',
            background: 'var(--cream-dark)',
            color: 'var(--ink-2)',
          }}
          onClick={() => { reset() }}
        >
          返回
        </button>
      </div>
    </div>
  )
}

export default BattleResult
