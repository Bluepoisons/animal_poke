import { describe, it, expect } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import {
  LoadingState,
  EmptyState,
  NoResultState,
  ErrorState,
  FatalState,
  OfflineState,
  StaleState,
} from './index'

describe('StateComponents', () => {
  const base = { title: 'Test Title' }
  const withBody = { ...base, body: 'Test body text' }
  const withRequestId = { ...base, requestId: 'req-abc-123' }
  const primary = { label: 'Retry', onClick: () => {} }

  it('LoadingState renders spinner and title', () => {
    render(<LoadingState {...base} />)
    expect(screen.getByTestId('loading-state')).toBeInTheDocument()
    expect(screen.getByText('Test Title')).toBeInTheDocument()
  })

  it('EmptyState renders icon and title', () => {
    render(<EmptyState {...withBody} />)
    expect(screen.getByTestId('empty-state')).toBeInTheDocument()
    expect(screen.getByText('Test body text')).toBeInTheDocument()
  })

  it('NoResultState renders search icon and title', () => {
    render(<NoResultState {...base} />)
    expect(screen.getByTestId('no-result-state')).toBeInTheDocument()
  })

  it('ErrorState renders retry button and request ID', () => {
    const onClick = vi.fn()
    render(<ErrorState {...withRequestId} primary={{ label: 'Retry', onClick, testId: 'error-retry' }} />)
    expect(screen.getByTestId('error-state')).toBeInTheDocument()
    expect(screen.getByTestId('state-request-id')).toHaveTextContent('req-abc-123')
    fireEvent.click(screen.getByTestId('error-retry'))
    expect(onClick).toHaveBeenCalledOnce()
  })

  it('FatalState renders without retry, shows request ID', () => {
    render(<FatalState {...withBody} requestId="fatal-456" />)
    expect(screen.getByTestId('fatal-state')).toBeInTheDocument()
    expect(screen.getByTestId('state-request-id')).toHaveTextContent('fatal-456')
    expect(screen.queryByTestId('state-action')).toBeNull()
  })

  it('OfflineState renders reconnect button', () => {
    const onClick = vi.fn()
    render(<OfflineState {...withBody} primary={{ label: 'Reconnect', onClick }} />)
    expect(screen.getByTestId('offline-state')).toBeInTheDocument()
    fireEvent.click(screen.getByTestId('state-action'))
    expect(onClick).toHaveBeenCalledOnce()
  })

  it('StaleState renders refresh action', () => {
    const onClick = vi.fn()
    render(<StaleState {...base} body="Data from 10 min ago" primary={{ label: 'Refresh', onClick }} />)
    expect(screen.getByTestId('stale-state')).toBeInTheDocument()
    expect(screen.getByText('Data from 10 min ago')).toBeInTheDocument()
  })

  it('StateContainer renders secondary action', () => {
    const onPrimary = vi.fn()
    const onSecondary = vi.fn()
    render(
      <ErrorState
        title="Error"
        body="Something went wrong"
        primary={{ label: 'Retry', onClick: onPrimary, testId: 'primary-btn' }}
        secondary={{ label: 'Go Back', onClick: onSecondary, testId: 'secondary-btn' }}
      />,
    )
    fireEvent.click(screen.getByTestId('secondary-btn'))
    expect(onSecondary).toHaveBeenCalledOnce()
    expect(onPrimary).not.toHaveBeenCalled()
  })

  it('request ID is hidden when not provided', () => {
    render(<ErrorState title="Error" />)
    expect(screen.queryByTestId('state-request-id')).toBeNull()
  })

  it('all seven components have role=status', () => {
    const components = [
      <LoadingState {...base} key="l" />,
      <EmptyState {...base} key="e" />,
      <NoResultState {...base} key="n" />,
      <ErrorState {...base} key="er" />,
      <FatalState {...base} key="f" />,
      <OfflineState {...base} key="o" />,
      <StaleState {...base} key="s" />,
    ]
    const { container } = render(<>{components}</>)
    const statuses = container.querySelectorAll('[role="status"]')
    expect(statuses.length).toBe(7)
  })
})
