import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { render, screen, cleanup, fireEvent, waitFor } from '@testing-library/react'
import { AppProviders } from '../../../providers/AppProviders'
import { grantConsent } from '../../../compliance'
import AnimalPokeApp from '../AnimalPokeApp'

describe('AP-124 prologue entry', () => {
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

  it('starts prologue from journal without camera', async () => {
    render(
      <AppProviders>
        <AnimalPokeApp />
      </AppProviders>,
    )
    fireEvent.click(await screen.findByTestId('tab-journal'))
    fireEvent.click(await screen.findByTestId('journal-start-prologue'))
    expect(await screen.findByTestId('prologue-screen')).toBeTruthy()
    expect(screen.getByTestId('presentation-player')).toBeTruthy()
    expect(screen.getByText(/无需相机/)).toBeTruthy()
  })
})
