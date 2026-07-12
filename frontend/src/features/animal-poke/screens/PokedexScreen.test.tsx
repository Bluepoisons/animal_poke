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
    expect(await screen.findByRole('button', { name: /cat.*稀有/i })).toBeTruthy()
    expect(screen.queryByTestId('loading-state')).toBeNull()
  })
})
