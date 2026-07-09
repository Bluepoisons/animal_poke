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
      <PageTitle title="BATTLE ARENA" />
      <div className="ap-versus-row">
        <AnimalIcon species="cat" size={112} />
        <div className="ap-vs">VS</div>
        <AnimalIcon species="dog" size={112} />
      </div>
      <div className="ap-hp ap-hp--player">
        <span style={{ width: `${playerHp}%` }} />
      </div>
      <div className="ap-hp ap-hp--enemy">
        <span style={{ width: `${enemyHp}%` }} />
      </div>
      <div className="ap-advantage">火 &gt; 草 · 光克暗</div>
      <BattleLog lines={logLines} />
      <div className="ap-battle-actions">
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
