import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { AnimalRecord } from '../../../db/types'
import { AnimalRepository } from '../../../db/repositories/animal-repository'
import { I18nProvider } from '../../../i18n'
import PokedexScreen from './PokedexScreen'

vi.mock('../../../db/repositories/animal-repository', () => ({
  AnimalRepository: {
    getAll: vi.fn(),
  },
}))

const record: AnimalRecord = {
  id: 'captured-cat',
  no: 'captured',
  rarity: 'rare',
  species: 'cat',
  unlocked: true,
  isUnlocked: 1,
  captureDate: '2026-07-12',
  location: '宁波',
  lat: 29.87,
  lng: 121.55,
  seed: 42,
	breed: 'British Shorthair',
  hp: 78,
  atk: 26,
  def: 42,
  spd: 30,
  className: 'Ranger',
  element: 'Wind',
  narrative: 'Luna follows the breeze and watches every path.',
  photoDataUrl: 'data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///ywAAAAAAQABAAACAUwAOw==',
}

const whaleRecord: AnimalRecord = {
  ...record,
  id: 'captured-whale',
  species: 'whale',
  nickname: 'Moby',
  breed: '蓝鲸',
  narrative: 'Moby 会沿着月光下的潮汐，寻找远方传来的歌声。',
}

const legacyUnknownRecord: AnimalRecord = {
  ...record,
  id: 'captured-legacy-unknown',
  species: undefined,
  speciesLabelZh: undefined,
  nickname: 'Nova',
  breed: undefined,
  narrative: 'Nova 会在落叶间寻找回家的方向。',
}

describe('PokedexScreen', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.clearAllMocks()
  })

  it('retries the repository load after a recoverable error', async () => {
    vi.mocked(AnimalRepository.getAll)
      .mockRejectedValueOnce(new Error('temporary IndexedDB error'))
      .mockResolvedValueOnce([record])

    render(
      <I18nProvider>
        <PokedexScreen onToast={vi.fn()} />
      </I18nProvider>,
    )

    expect(await screen.findByTestId('error-state')).toBeTruthy()
    fireEvent.click(screen.getByTestId('state-action'))

    await waitFor(() => expect(AnimalRepository.getAll).toHaveBeenCalledTimes(2))
    expect(await screen.findByRole('button', { name: /sunny.*稀有/i })).toBeTruthy()
    expect(screen.queryByTestId('loading-state')).toBeNull()
  })

  it('shows a pet profile with the local capture photo and value stats', async () => {
    vi.mocked(AnimalRepository.getAll).mockResolvedValueOnce([record])
    const onStartAdventure = vi.fn()

    render(
      <I18nProvider>
        <PokedexScreen onToast={vi.fn()} onStartAdventure={onStartAdventure} />
      </I18nProvider>,
    )

    const card = await screen.findByRole('button', { name: /sunny.*稀有/i })
    expect(screen.getByText('生命 78 · 攻击 26')).toBeTruthy()
    expect(screen.getByAltText('Sunny 的照片')).toBeTruthy()

    fireEvent.click(card)
    expect(await screen.findByText('防御')).toBeTruthy()
    expect(screen.getByText('42')).toBeTruthy()
		expect(screen.getByText(/Sunny 是一位风属性的游侠猫伙伴/)).toBeTruthy()
		expect(screen.getAllByText(/英国短毛猫/).length).toBeGreaterThan(0)
		expect(screen.queryByText(/Ranger|Wind|British Shorthair/)).toBeNull()

    fireEvent.click(screen.getByRole('button', { name: '带它去探险' }))
    expect(onStartAdventure).toHaveBeenCalledWith(record.id)
  })

  it('shows the concrete Chinese name for a broad fallback animal', async () => {
    vi.mocked(AnimalRepository.getAll).mockResolvedValueOnce([{
      ...record,
      id: 'captured-red-fox',
      species: 'other_animal',
      speciesLabelZh: '赤狐',
      nickname: 'Mika',
      breed: '赤狐',
      narrative: 'Mika 会在幻想森林里循着月光寻找风铃。',
    }])
    render(
      <I18nProvider>
        <PokedexScreen onToast={vi.fn()} />
      </I18nProvider>,
    )
    const card = await screen.findByRole('button', { name: /Mika.*稀有/i })
    expect(screen.getByText(/赤狐 · 游侠/)).toBeTruthy()
    fireEvent.click(card)
    expect(await screen.findByText(/赤狐 · 稀有/)).toBeTruthy()
  })

  it('filters records by descriptor group and sends unknown IDs to other', async () => {
    vi.mocked(AnimalRepository.getAll).mockResolvedValueOnce([record, whaleRecord, legacyUnknownRecord])

    render(
      <I18nProvider>
        <PokedexScreen onToast={vi.fn()} />
      </I18nProvider>,
    )

    const companionFilter = await screen.findByRole('button', { name: '伙伴动物' })
    const aquaticFilter = screen.getByRole('button', { name: '水生动物' })
    const otherFilter = screen.getByRole('button', { name: '其他动物' })
    expect(screen.queryByText(/\bwhale\b/i)).toBeNull()
    expect(screen.queryByRole('button', { name: '鲸' })).toBeNull()

    fireEvent.click(aquaticFilter)
    expect(await screen.findByRole('button', { name: /Moby.*稀有/i })).toBeTruthy()
    expect(screen.queryByRole('button', { name: /Sunny.*稀有/i })).toBeNull()

    fireEvent.click(companionFilter)
    expect(await screen.findByRole('button', { name: /Sunny.*稀有/i })).toBeTruthy()
    expect(screen.queryByRole('button', { name: /Moby.*稀有/i })).toBeNull()

    fireEvent.click(otherFilter)
    expect(await screen.findByRole('button', { name: /Nova.*稀有/i })).toBeTruthy()
    expect(screen.getAllByText(/动物伙伴/).length).toBeGreaterThan(0)
    expect(screen.queryByText(/^猫$/)).toBeNull()
  })
})
