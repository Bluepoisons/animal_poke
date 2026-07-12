import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { render, screen, cleanup, fireEvent } from '@testing-library/react'
import { AppProviders } from '../../../providers/AppProviders'
import { grantConsent } from '../../../compliance'
import AnimalPokeApp from '../AnimalPokeApp'

describe('AP-121 journal UI entry', () => {
  beforeEach(() => {
    localStorage.clear()
    sessionStorage.clear()
    localStorage.setItem(
      'animal-poke-onboarding-v1',
      JSON.stringify({ step: 'done', skipped: true, completedAt: Date.now() }),
    )
    grantConsent()
    location.hash = ''
  })
  afterEach(() => cleanup())

  it('opens journal from tab and shows goal / last event within one view', async () => {
    render(
      <AppProviders>
        <AnimalPokeApp />
      </AppProviders>,
    )
    fireEvent.click(await screen.findByTestId('tab-journal'))
    expect(await screen.findByTestId('journal-screen')).toBeTruthy()
    expect(screen.getByTestId('journal-current-goal').textContent?.length).toBeGreaterThan(4)
    expect(screen.getByTestId('journal-last-event').textContent?.length).toBeGreaterThan(4)
    fireEvent.click(screen.getByTestId('journal-tab-clues'))
    expect(screen.getByTestId('journal-clue-board')).toBeTruthy()
  })
})
