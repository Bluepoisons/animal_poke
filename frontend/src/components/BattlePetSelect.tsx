import React from 'react'
import { useAnimalStore } from '../hooks/useAnimalStore'
import { useBattle } from '../battle/useBattle'
import { useStamina } from '../stamina/useStamina'
import { cardEntryToBattlePet, applyStatusMultiplier } from '../battle/logic'
import { BATTLE_STAMINA_COST } from '../battle/constants'
import { SPECIES_DEFS, RARITY_NAMES, RARITY_COLORS } from '../types'
import type { CardEntry } from '../battle/types'
import { ELEMENT_TYPES } from '../battle/constants'
import { useStatus } from '../status/useStatus'

/** 宠物选择界面 */
const BattlePetSelect: React.FC = () => {
  const { animals } = useAnimalStore()
  const { selectPet, startMatching, state } = useBattle()
  const stamina = useStamina()
  const statusCtx = useStatus()

  const availablePets = animals.filter(a => a.unlocked)
  const hasEnoughStamina = stamina.state.currentStamina >= BATTLE_STAMINA_COST

  const handleConfirm = (entry: CardEntry) => {
    if (!hasEnoughStamina) return
    const success = selectPet(entry)
    if (success) {
      startMatching()
    }
  }

  // 元素颜色
  const elementColor: Record<string, string> = {
    fire: '#E55934',
    water: '#4D8CFF',
    grass: '#4DCC4D',
    light: '#FFCC33',
    dark: '#A65CF2',
  }
  const elementEmoji: Record<string, string> = {
    fire: '🔥',
    water: '💧',
    grass: '🌿',
    light: '✨',
    dark: '🌑',
  }

  return (
    <div style={{ padding: '16px 12px', height: '100%', overflowY: 'auto' }}>
      <div style={{ textAlign: 'center', marginBottom: 16 }}>
        <h2 style={{ fontSize: 18, fontWeight: 700, color: 'var(--ink)' }}>选择出战宠物</h2>
        <div style={{ fontSize: 12, color: hasEnoughStamina ? 'var(--success)' : 'var(--warn)', marginTop: 4 }}>
          {hasEnoughStamina
            ? `消耗 ${BATTLE_STAMINA_COST} 体力开始战斗`
            : `体力不足！需要 ${BATTLE_STAMINA_COST} 体力`}
        </div>
      </div>

      {!hasEnoughStamina && (
        <div style={{
          textAlign: 'center',
          padding: 24,
          background: 'rgba(229,89,52,0.08)',
          borderRadius: 'var(--radius-lg)',
          color: 'var(--warn)',
          fontWeight: 600,
        }}>
          体力不足，无法开始战斗
        </div>
      )}

      <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
        {availablePets.map((entry) => {
          const species = entry.species ?? 'cat'
          const speciesDef = SPECIES_DEFS[species]
          const element = ELEMENT_TYPES[((entry.seed % 5) + 5) % 5]
          const pet = state.weather ? cardEntryToBattlePet(entry, state.weather) : null

          return (
            <div key={entry.id} style={{
              display: 'flex',
              alignItems: 'center',
              gap: 12,
              background: 'var(--white)',
              borderRadius: 'var(--radius-md)',
              padding: '10px 14px',
              boxShadow: 'var(--shadow-card)',
            }}>
              {/* 头像 */}
              <div style={{
                fontSize: 32,
                width: 48,
                textAlign: 'center',
                borderRadius: 'var(--radius-sm)',
                background: 'var(--orange-50)',
              }}>
                {speciesDef.emoji}
              </div>

              {/* 属性信息 */}
              <div style={{ flex: 1 }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 6, flexWrap: 'wrap' }}>
                  <span style={{ fontWeight: 700, fontSize: 14 }}>{speciesDef.name}</span>
                  <span style={{
                    fontSize: 10,
                    fontWeight: 600,
                    color: RARITY_COLORS[entry.rarity],
                    background: 'var(--orange-50)',
                    borderRadius: 8,
                    padding: '2px 6px',
                  }}>
                    {RARITY_NAMES[entry.rarity]}
                  </span>
                  <span style={{
                    fontSize: 10,
                    fontWeight: 600,
                    color: elementColor[element],
                  }}>
                    {elementEmoji[element]} {element}
                  </span>
                  {statusCtx.getPetStatusDisplay(entry.id).map((s, i) => {
                    if (s.type === 'normal') return null
                    return (
                      <span
                        key={i}
                        style={{
                          fontSize: 10,
                          fontWeight: 600,
                          color: s.color,
                          background: 'var(--orange-50)',
                          borderRadius: 8,
                          padding: '2px 6px',
                        }}
                      >
                        {s.emoji} {s.label}{s.remainingDays ? ` ${s.remainingDays}天` : ''}
                      </span>
                    )
                  })}
                </div>
                {pet && (() => {
                  const mod = statusCtx.getStatModifier(entry.id)
                  const modifiedPet = mod !== 1.0
                    ? { ...pet, stats: applyStatusMultiplier(pet.stats, mod) }
                    : pet
                  return (
                    <div style={{ fontSize: 11, color: 'var(--ink-3)', marginTop: 3, display: 'flex', gap: 8 }}>
                      <span>HP {modifiedPet.stats.hp}{mod !== 1.0 && <span style={{ color: 'var(--warn)' }}>↓</span>}</span>
                      <span>ATK {modifiedPet.stats.atk}{mod !== 1.0 && <span style={{ color: 'var(--warn)' }}>↓</span>}</span>
                      <span>DEF {modifiedPet.stats.def}{mod !== 1.0 && <span style={{ color: 'var(--warn)' }}>↓</span>}</span>
                      <span>SPD {modifiedPet.stats.spd}{mod !== 1.0 && <span style={{ color: 'var(--warn)' }}>↓</span>}</span>
                    </div>
                  )
                })()}
              </div>

              {/* 出战按钮 */}
              <button
                className="btn btn-primary"
                style={{ fontSize: 12, padding: '6px 12px' }}
                onClick={() => handleConfirm(entry)}
                disabled={!hasEnoughStamina}
              >
                出战
              </button>
            </div>
          )
        })}
      </div>

      {availablePets.length === 0 && (
        <div style={{ textAlign: 'center', color: 'var(--ink-3)', padding: 32 }}>
          没有已解锁的宠物，先去捕获吧！
        </div>
      )}
    </div>
  )
}

export default BattlePetSelect
