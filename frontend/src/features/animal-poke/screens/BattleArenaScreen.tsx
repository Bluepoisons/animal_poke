import { useState } from 'react'
import type { Strategy } from '../data/types'
import PageTitle from '../components/PageTitle'
import AnimalIcon from '../components/AnimalIcon'
import BattleLog from '../components/BattleLog'

interface StrategyConfig {
  label: string
  playerDamage: number
  enemyDamage: number
  log: [string, string]
}

const strategies: Record<Strategy, StrategyConfig> = {
  aggressive: {
    label: '激进',
    playerDamage: 24,
    enemyDamage: 12,
    log: ['猫 使用 激进策略，暴击 x2', '获得金币 +70 · 掉落诱饵'],
  },
  balanced: {
    label: '平衡',
    playerDamage: 16,
    enemyDamage: 8,
    log: ['猫 使用 平衡策略，稳定命中', '获得金币 +45'],
  },
  defensive: {
    label: '防守',
    playerDamage: 9,
    enemyDamage: 3,
    log: ['猫 使用 防守策略，减少伤害', '获得金币 +25'],
  },
}

export default function BattleArenaScreen() {
  const [playerHp, setPlayerHp] = useState(100)
  const [enemyHp, setEnemyHp] = useState(100)
  const [activeStrategy, setActiveStrategy] = useState<Strategy>('aggressive')
  const [logLines, setLogLines] = useState<string[]>([
    '猫 使用 激进策略，暴击 x2',
    '获得金币 +70 · 掉落诱饵',
  ])

  const handleStrategy = (strategy: Strategy) => {
    const config = strategies[strategy]
    setActiveStrategy(strategy)
    setPlayerHp((prev) => Math.max(0, prev - config.enemyDamage))
    setEnemyHp((prev) => Math.max(0, prev - config.playerDamage))
    setLogLines([config.log[0], config.log[1]])
  }

  return (
    <div className="ap-screen">
      <PageTitle
        title="BATTLE"
        subtitle="ARENA · 手账对战页"
        rightText="回合中"
        rightTone="blue"
      />

      <div className="ap-versus-row">
        <div className="ap-fighter">
          <div className="ap-animal-badge ap-animal-badge--pink" style={{ width: 112, height: 112 }}>
            <AnimalIcon species="cat" size={88} />
          </div>
          <span className="ap-fighter__name">我的猫</span>
        </div>
        <div className="ap-vs">VS</div>
        <div className="ap-fighter">
          <div className="ap-animal-badge ap-animal-badge--blue" style={{ width: 112, height: 112 }}>
            <AnimalIcon species="dog" size={88} />
          </div>
          <span className="ap-fighter__name">野生狗</span>
        </div>
      </div>

      <div className="ap-hp-row">
        <div className="ap-hp-meta">
          <span>我方 HP</span>
          <span>{playerHp}</span>
        </div>
        <div className="ap-hp ap-hp--player" aria-label={`我方血量 ${playerHp}`}>
          <span style={{ width: `${playerHp}%` }} />
        </div>
        <div className="ap-hp-meta">
          <span>敌方 HP</span>
          <span>{enemyHp}</span>
        </div>
        <div className="ap-hp ap-hp--enemy" aria-label={`敌方血量 ${enemyHp}`}>
          <span style={{ width: `${enemyHp}%` }} />
        </div>
      </div>

      <div className="ap-advantage">火 &gt; 草 · 光克暗</div>

      <BattleLog lines={logLines} />

      <div className="ap-battle-actions" aria-label="战斗策略">
        {(Object.keys(strategies) as Strategy[]).map((key) => (
          <button
            key={key}
            className={activeStrategy === key ? 'is-active' : ''}
            onClick={() => handleStrategy(key)}
            type="button"
          >
            {strategies[key].label}
          </button>
        ))}
      </div>
    </div>
  )
}
