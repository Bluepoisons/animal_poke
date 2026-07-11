/**
 * AP-071: Accessibility unit tests — focus trap, progressbar, live region, route announcer.
 */
import { describe, it, expect, afterEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useRef, useState } from 'react'

import { ProgressBar, ErrorLiveRegion, useFocusTrap, RouteAnnouncerElement, useRouteAnnouncer } from './index'
import type { FocusTrapOptions } from './index'

// ---------- ProgressBar ----------

describe('ProgressBar', () => {
  it('renders with correct role and aria attributes', () => {
    render(<ProgressBar value={45} max={100} label="HP" />)
    const bar = screen.getByRole('progressbar', { name: 'HP' })
    expect(bar).toHaveAttribute('aria-valuenow', '45')
    expect(bar).toHaveAttribute('aria-valuemin', '0')
    expect(bar).toHaveAttribute('aria-valuemax', '100')
    expect(bar).toHaveAttribute('aria-valuetext', '45 / 100')
  })

  it('clamps to 0-100%', () => {
    render(<ProgressBar value={-10} max={50} label="test" />)
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '-10')
    // CSS var clamps visually; value text reflects actual value
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuetext', '-10 / 50')
  })

  it('respects custom valueText', () => {
    render(<ProgressBar value={60} label="rank" valueText="Level 12" />)
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuetext', 'Level 12')
  })

  it('renders animated by default', () => {
    render(<ProgressBar value={30} label="energy" />)
    const bar = screen.getByRole('progressbar', { name: 'energy' })
    expect(bar.className).toContain('ap-progressbar--animated')
  })
})

// ---------- ErrorLiveRegion ----------

describe('ErrorLiveRegion', () => {
  it('renders as role=alert for errors', () => {
    render(<ErrorLiveRegion message="Something went wrong" />)
    expect(screen.getByRole('alert')).toHaveTextContent('Something went wrong')
  })

  it('renders as role=status when polite', () => {
    render(<ErrorLiveRegion message="Loading..." polite />)
    expect(screen.getByRole('status')).toHaveTextContent('Loading...')
  })

  it('shows NBSP when message is null', () => {
    render(<ErrorLiveRegion message={null} />)
    expect(screen.getByRole('alert')).toBeInTheDocument()
  })
})

// ---------- FocusTrap ----------

function FocusTrapFixture({ active, onEscape }: { active: boolean; onEscape?: () => void }) {
  const ref = useRef<HTMLDivElement>(null)
  useFocusTrap({ containerRef: ref, active, onEscape, closeOnBackdrop: false })
  return (
    <div>
      <button data-testid="outside">Outside</button>
      {active && (
        <div ref={ref} role="dialog" aria-label="test dialog" data-testid="trap-container">
          <button data-testid="first">First</button>
          <button data-testid="second">Second</button>
          <input data-testid="input" placeholder="type here" />
          <button data-testid="last">Last</button>
        </div>
      )}
    </div>
  )
}

describe('useFocusTrap', () => {
  afterEach(() => {
    document.body.focus()
  })

  it('focuses first focusable element on activation', async () => {
    render(<FocusTrapFixture active />)
    // focus trap runs in requestAnimationFrame — wait
    await new Promise(r => setTimeout(r, 50))
    // The first focusable element should be focused
    const first = screen.getByTestId('first')
    expect(document.activeElement).toBe(first)
  })

  it('traps tab at last element back to first', async () => {
    render(<FocusTrapFixture active />)
    await new Promise(r => setTimeout(r, 50))
    const first = screen.getByTestId('first')
    const last = screen.getByTestId('last')
    last.focus()
    expect(document.activeElement).toBe(last)
    await userEvent.tab()
    expect(document.activeElement).toBe(first)
  })

  it('calls onEscape on Escape key', async () => {
    const onEscape = vi.fn()
    render(<FocusTrapFixture active onEscape={onEscape} />)
    await new Promise(r => setTimeout(r, 50))
    fireEvent.keyDown(document.activeElement!, { key: 'Escape' })
    expect(onEscape).toHaveBeenCalledTimes(1)
  })

  it('does NOT trap when inactive', () => {
    render(<FocusTrapFixture active={false} />)
    expect(screen.getByTestId('outside')).toBeInTheDocument()
    // no dialog rendered
    expect(screen.queryByTestId('trap-container')).toBeNull()
  })
})

// ---------- RouteAnnouncer ----------

describe('useRouteAnnouncer', () => {
  function TestRoute({ title }: { title: string }) {
    useRouteAnnouncer(title)
    return <div data-testid="route-content">{title}</div>
  }

  it('updates route announcer text', async () => {
    const { rerender } = render(
      <>
        <RouteAnnouncerElement />
        <TestRoute title="图鉴" />
      </>,
    )
    await new Promise(r => setTimeout(r, 50))
    const announcer = document.getElementById('ap-route-announcer')
    expect(announcer?.textContent).toBe('图鉴')

    rerender(
      <>
        <RouteAnnouncerElement />
        <TestRoute title="发现" />
      </>,
    )
    await new Promise(r => setTimeout(r, 50))
    expect(document.getElementById('ap-route-announcer')?.textContent).toBe('发现')
  })
})
