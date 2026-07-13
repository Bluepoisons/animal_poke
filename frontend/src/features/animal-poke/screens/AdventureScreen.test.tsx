import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { AnimalRecord } from '../../../db/types'
import { AnimalRepository } from '../../../db/repositories/animal-repository'
import {
  completeAdventure,
  fetchAdventureCompanion,
  fetchAdventureHistory,
  generateAdventure,
} from '../adventureApi'
import AdventureScreen from './AdventureScreen'

vi.mock('../../../db/repositories/animal-repository', () => ({
  AnimalRepository: {
    getUnlocked: vi.fn(),
  },
}))

vi.mock('../adventureApi', async () => {
  const actual = await vi.importActual<typeof import('../adventureApi')>('../adventureApi')
  return {
    ...actual,
    generateAdventure: vi.fn(),
    completeAdventure: vi.fn(),
    fetchAdventureCompanion: vi.fn(),
    fetchAdventureHistory: vi.fn(),
  }
})

const pet: AnimalRecord = {
  id: '550e8400-e29b-41d4-a716-446655440000',
  uuid: '550e8400-e29b-41d4-a716-446655440000',
  no: '550e8400',
  rarity: 'rare',
  species: 'other_animal',
  speciesLabelZh: '赤狐',
  unlocked: true,
  isUnlocked: 1,
  captureDate: '2026-07-13',
  location: '宁波',
  lat: 0,
  lng: 0,
  seed: 7,
  breed: '赤狐',
  className: 'Ranger',
  element: 'Wind',
  hp: 68,
  atk: 27,
  def: 30,
  spd: 42,
}

const story = {
  adventure_id: 'adventure-1',
  theme: 'mistwood' as const,
  title: '雾灯森林的风铃',
  location: '萤光岔路',
  opening: 'Luna踩亮了林间第一块苔藓。风属性的光点一路跟在身后。',
  encounter_title: '沉默的风铃精灵',
  encounter: '一只风铃精灵忘记了自己的旋律。它把三枚发光音符交到你们面前。',
  companion_line: 'Luna抬头望着你，尾巴轻轻扫过光点。',
  choices: [
    { id: 'courage' as const, label: '先唱第一声', description: '和伙伴一起勇敢打破寂静' },
    { id: 'curiosity' as const, label: '寻找旧旋律', description: '观察四周隐藏的音符规律' },
    { id: 'kindness' as const, label: '陪它慢慢想', description: '先安静陪伴再轻轻回应' },
  ],
  fiction: true as const,
  disclaimer: 'AI 生成的幻想冒险，不代表真实动物的经历、情绪或需求',
  source: 'ai' as const,
  prompt_version: 'companion-adventure-zh-v2',
}

describe('AdventureScreen', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(AnimalRepository.getUnlocked).mockResolvedValue([pet])
    vi.mocked(fetchAdventureCompanion).mockResolvedValue({
      animal_uuid: pet.id,
      bond_xp: 12,
      bond_level: 1,
      decor_stage: 0,
      title: '初识伙伴',
    })
    vi.mocked(fetchAdventureHistory).mockResolvedValue([])
    vi.mocked(generateAdventure).mockResolvedValue(story)
    vi.mocked(completeAdventure).mockResolvedValue({
      adventure_id: story.adventure_id,
      status: 'completed',
      choice: {
        ...story.choices[1],
        outcome: '你们找回了旋律，也读懂了彼此等待的节奏。',
        bond_delta: 6,
      },
      outcome: '你们找回了旋律，也读懂了彼此等待的节奏。',
      souvenir: { name: '风铃叶', description: '叶片会奏出你们共同找到的旋律。' },
      companion: {
        animal_uuid: pet.id,
        bond_xp: 18,
        bond_level: 1,
        decor_stage: 0,
      },
      idempotent: false,
    })
  })

  it('uses Chinese pet metadata and completes a controllable RPG encounter', async () => {
    render(<AdventureScreen onToast={vi.fn()} onOpenCollection={vi.fn()} />)

    expect(await screen.findByTestId('adventure-screen')).toBeTruthy()
    expect(screen.getByText('赤狐 · 游侠 / 风属性')).toBeTruthy()
    expect(screen.queryByText(/other_animal|Ranger|Wind/)).toBeNull()

    fireEvent.click(screen.getByTestId('adventure-start'))
    expect(await screen.findByTestId('adventure-run')).toBeTruthy()

    const right = screen.getByRole('button', { name: '向右移动' })
    const up = screen.getByRole('button', { name: '向上移动' })
    fireEvent.click(right)
    fireEvent.click(right)
    fireEvent.click(right)
    fireEvent.click(up)
    fireEvent.click(up)

    expect(await screen.findByTestId('adventure-encounter')).toBeTruthy()
    expect(screen.queryByText(/共同找到的旋律/)).toBeNull()

    fireEvent.click(screen.getByTestId('adventure-choice-curiosity'))
    expect(await screen.findByTestId('adventure-result')).toBeTruthy()
    expect(screen.getByText('风铃叶')).toBeTruthy()
    expect(screen.getByText(/羁绊加深/)).toBeTruthy()
    await waitFor(() => expect(completeAdventure).toHaveBeenCalledWith('adventure-1', 'curiosity'))
  })

  it('keeps a legacy pet without species read-only and never starts generation', async () => {
    vi.mocked(AnimalRepository.getUnlocked).mockResolvedValueOnce([{
      ...pet,
      species: undefined,
      speciesLabelZh: undefined,
      breed: undefined,
    }])

    render(<AdventureScreen onToast={vi.fn()} onOpenCollection={vi.fn()} />)

    expect(await screen.findByText('动物伙伴')).toBeTruthy()
    expect(screen.getByText(/动物伙伴 · 游侠 \/ 风属性/)).toBeTruthy()
    expect(screen.queryByText(/^猫$/)).toBeNull()
    expect(screen.getByText('需确认物种后才能探险')).toBeTruthy()

    const startButton = screen.getByTestId('adventure-start')
    expect(startButton).toBeDisabled()
    fireEvent.click(startButton)
    expect(generateAdventure).not.toHaveBeenCalled()
  })
})
