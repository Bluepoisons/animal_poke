import { useEffect, useMemo } from 'react'
import PageTitle from '../components/PageTitle'
import AnimalIcon from '../components/AnimalIcon'
import BattleLog from '../components/BattleLog'
import { useBattle } from '../../../battle/useBattle'
import type { CardEntry, SpeciesType } from '../../../types'
import type { StrategyType } from '../../../battle/types'

const strategies: { id: StrategyType; label: string }[] = [
  { id: 'aggressive', label: '激进' },
  { id: 'balanced', label: '平衡' },
  { id: 'defensive', label: '防守' },
]

const DEMO_PET: CardEntry = {
  id: 'battle-demo-cat',
  no: '#DEMO',
  rarity: 'uncommon',
  species: 'cat',
  unlocked: true,
  captureDate: new Date().toISOString().slice(0, 10),
  location: '训练场',
  lat: 0,
  lng: 0,
  seed: 1,
}

export default function BattleArenaScreen() {
  const battle = useBattle()
  const { state } = battle

  // 进入选宠并自动用演示宠开始（生产可改为图鉴选择）
  useEffect(() => {
    if (state.phase === 'idle') {
      battle.enterSelect()
    }
  }, [state.phase, battle])

  useEffect(() => {
    if (state.phase === 'selecting' && !state.playerPet) {
      battle.selectPet(DEMO_PET)
      battle.startMatching()
    }
  }, [state.phase, state.playerPet, battle])

  const playerSpecies = (state.playerPet?.species || 'cat') as SpeciesType
  const enemySpecies = (state.enemyPet?.species || 'dog') as SpeciesType
  const playerHpPct = state.playerPet
    ? Math.round((state.playerPet.currentHp / state.playerPet.stats.hp) * 100)
    : 100
  const enemyHpPct = state.enemyPet
    ? Math.round((state.enemyPet.currentHp / state.enemyPet.stats.hp) * 100)
    : 100

  const logLines = useMemo(
    () => state.log.slice(-6).map((l) => l.text),
    [state.log],
  )

  const phaseLabel =
    state.phase === 'idle'
      ? '准备'
      : state.phase === 'selecting'
        ? '选宠'
        : state.phase === 'matching'
          ? '匹配中'
          : state.phase === 'battling'
            ? `第 ${state.round} 回合`
            : state.phase === 'result'
              ? `结果 ${state.result ?? ''}`
              : state.phase

  const handleStrategy = (strategy: StrategyType) => {
    if (state.phase === 'result') return
    battle.setStrategy(strategy)
    battle.executeNextRound()
  }

  return (
    <div className="ap-screen">
      <PageTitle
        title="BATTLE"
        subtitle="ARENA · BattleContext 权威结算"
        rightText={phaseLabel}
        rightTone="blue"
      />

      <div className="ap-versus-row">
        <div className="ap-fighter">
          <div className="ap-animal-badge ap-animal-badge--pink" style={{ width: 112, height: 112 }}>
            <AnimalIcon species={playerSpecies} size={88} />
          </div>
          <span className="ap-fighter__name">我的{playerSpecies}</span>
        </div>
        <div className="ap-vs">VS</div>
        <div className="ap-fighter">
          <div className="ap-animal-badge ap-animal-badge--blue" style={{ width: 112, height: 112 }}>
            <AnimalIcon species={enemySpecies} size={88} />
          </div>
          <span className="ap-fighter__name">野生{enemySpecies}</span>
        </div>
      </div>

      <div className="ap-hp-row">
        <div className="ap-hp">
          <span>我方 HP</span>
          <div className="ap-hp__bar">
            <i style={{ width: `${playerHpPct}%` }} />
          </div>
        </div>
        <div className="ap-hp">
          <span>敌方 HP</span>
          <div className="ap-hp__bar">
            <i style={{ width: `${enemyHpPct}%` }} />
          </div>
        </div>
      </div>

      <div className="ap-strategy-row" aria-label="策略">
        {strategies.map((s) => (
          <button
            key={s.id}
            type="button"
            className={state.strategy === s.id ? 'is-active' : ''}
            disabled={state.phase === 'result' || state.phase === 'idle'}
            onClick={() => handleStrategy(s.id)}
          >
            {s.label}
          </button>
        ))}
      </div>

      <BattleLog lines={logLines.length ? logLines : ['等待战斗…']} />

      {state.phase === 'result' && (
        <div style={{ padding: 12, display: 'flex', gap: 8 }}>
          <button type="button" className="ap-map-chip" onClick={() => battle.finishBattle()}>
            领取结算
          </button>
          <button type="button" className="ap-map-chip" onClick={() => battle.reset()}>
            再来一局
          </button>
        </div>
      )}
    </div>
  )
}
