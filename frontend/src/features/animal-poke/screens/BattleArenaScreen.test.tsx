import { fireEvent, render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useBattle } from '../../../battle/useBattle'
import type { BattleContextValue, BattlePet } from '../../../battle/types'
import { MAX_ENERGY } from '../../../battle/constants'
import { I18nProvider } from '../../../i18n'
import BattleArenaScreen from './BattleArenaScreen'

vi.mock('../../../battle/useBattle', () => ({ useBattle: vi.fn() }))

function pet(id: string, isPlayer: boolean, energy: number): BattlePet {
  const stats = { hp: 100, atk: 25, def: 15, spd: 20, crit: 5, eva: 5 }
  return {
    id,
    name: id,
    emoji: isPlayer ? '🐱' : '🐶',
    species: isPlayer ? 'cat' : 'dog',
    rarity: 'uncommon',
    element: 'fire',
    stats,
    baseStats: stats,
    currentHp: 100,
    energy,
    isPlayer,
    strategy: 'balanced',
  }
}

describe('BattleArenaScreen', () => {
  beforeEach(() => vi.clearAllMocks())

  it('exposes the ultimate action when auto battle pauses at full energy', () => {
    const useUltimate = vi.fn(() => true)
    const battle: BattleContextValue = {
      state: {
        phase: 'battling',
        playerPet: pet('player', true, MAX_ENERGY),
        enemyPet: pet('enemy', false, 0),
        round: 4,
        maxRounds: 30,
        log: [],
        result: null,
        rewards: null,
        strategy: 'balanced',
        weather: 'sunny',
        isAutoPlaying: true,
      },
      enterSelect: vi.fn(),
      selectPet: vi.fn(() => true),
      startMatching: vi.fn(),
      executeNextRound: vi.fn(),
      useUltimate,
      setStrategy: vi.fn(),
      useBattleItem: vi.fn(() => true),
      toggleAutoPlay: vi.fn(),
      finishBattle: vi.fn(),
      reset: vi.fn(),
    }
    vi.mocked(useBattle).mockReturnValue(battle)

    render(
      <I18nProvider>
        <BattleArenaScreen />
      </I18nProvider>,
    )

    const ultimate = screen.getByTestId('battle-ultimate')
    expect(ultimate).toBeEnabled()
    expect(screen.getByText('我的猫')).toBeInTheDocument()
    expect(screen.getByText('对手狗')).toBeInTheDocument()
    expect(screen.getByText('伙伴对战')).toBeInTheDocument()
    fireEvent.click(ultimate)
    expect(useUltimate).toHaveBeenCalledTimes(1)
  })

  it('shows a neutral companion when battle species is missing', () => {
    const battle: BattleContextValue = {
      state: {
        phase: 'matching',
        playerPet: null,
        enemyPet: null,
        round: 0,
        maxRounds: 30,
        log: [],
        result: null,
        rewards: null,
        strategy: 'balanced',
        weather: 'sunny',
        isAutoPlaying: false,
      },
      enterSelect: vi.fn(),
      selectPet: vi.fn(() => true),
      startMatching: vi.fn(),
      executeNextRound: vi.fn(),
      useUltimate: vi.fn(() => false),
      setStrategy: vi.fn(),
      useBattleItem: vi.fn(() => true),
      toggleAutoPlay: vi.fn(),
      finishBattle: vi.fn(),
      reset: vi.fn(),
    }
    vi.mocked(useBattle).mockReturnValue(battle)

    render(
      <I18nProvider>
        <BattleArenaScreen />
      </I18nProvider>,
    )

    expect(screen.getByText('我的动物伙伴')).toBeInTheDocument()
    expect(screen.getByText('对手动物伙伴')).toBeInTheDocument()
    expect(screen.queryByText(/我的猫|对手狗/)).toBeNull()
  })
})
