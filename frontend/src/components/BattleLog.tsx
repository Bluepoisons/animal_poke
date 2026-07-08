import React, { useEffect, useRef } from 'react'
import type { BattleLogEntry } from '../battle/types'

interface BattleLogProps {
  logs: BattleLogEntry[]
}

/** 战斗日志面板（自动滚动到最新） */
const BattleLog: React.FC<BattleLogProps> = ({ logs }) => {
  const containerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (containerRef.current) {
      containerRef.current.scrollTop = containerRef.current.scrollHeight
    }
  }, [logs])

  // 不同类型日志颜色
  const logColor = (type: BattleLogEntry['type']): string => {
    switch (type) {
      case 'crit': return '#FFB300'       // 金色
      case 'miss': return 'var(--ink-3)'  // 灰色
      case 'ultimate': return '#A65CF2'   // 紫色
      case 'item': return 'var(--success)' // 绿色
      case 'system': return 'var(--ink-3)' // 灰色
      default: return 'var(--ink-2)'       // 普通棕
    }
  }

  return (
    <div
      ref={containerRef}
      style={{
        height: 120,
        overflowY: 'auto',
        background: 'rgba(0,0,0,0.06)',
        borderRadius: 'var(--radius-md)',
        padding: '8px 12px',
        fontSize: 12,
        lineHeight: 1.6,
      }}
    >
      {logs.length === 0 && (
        <div style={{ color: 'var(--ink-3)', textAlign: 'center' }}>等待战斗开始...</div>
      )}
      {logs.map((log, i) => (
        <div key={i} style={{ color: logColor(log.type), fontWeight: log.type === 'crit' || log.type === 'ultimate' ? 700 : 400 }}>
          {log.text}
        </div>
      ))}
    </div>
  )
}

export default BattleLog
