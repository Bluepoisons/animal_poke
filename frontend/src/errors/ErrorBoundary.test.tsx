import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import React from 'react'
import { ErrorBoundary } from './ErrorBoundary'

// Mock reporter to avoid network calls
vi.mock('./reporter', () => ({
  reportError: vi.fn().mockResolvedValue(undefined),
}))

function ThrowComponent({ shouldThrow }: { shouldThrow?: boolean }) {
  if (shouldThrow) throw new Error('test error')
  return <div>OK</div>
}

describe('ErrorBoundary', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders children when no error', () => {
    render(
      <ErrorBoundary name="test">
        <div>hello</div>
      </ErrorBoundary>
    )
    expect(screen.getByText('hello')).toBeInTheDocument()
  })

  it('renders default fallback on error', () => {
    render(
      <ErrorBoundary name="test">
        <ThrowComponent shouldThrow />
      </ErrorBoundary>
    )
    expect(screen.getByText('😵')).toBeInTheDocument()
    expect(screen.getByText('页面出了点问题')).toBeInTheDocument()
  })

  it('renders custom fallback when provided', () => {
    render(
      <ErrorBoundary name="test" fallback={<div>Custom Error UI</div>}>
        <ThrowComponent shouldThrow />
      </ErrorBoundary>
    )
    expect(screen.getByText('Custom Error UI')).toBeInTheDocument()
  })

  it('renders functional fallback with error and retry', () => {
    render(
      <ErrorBoundary
        name="test"
        fallback={(error, retry) => (
          <div>
            <span>{error.message}</span>
            <button onClick={retry}>Retry</button>
          </div>
        )}
      >
        <ThrowComponent shouldThrow />
      </ErrorBoundary>
    )
    expect(screen.getByText('test error')).toBeInTheDocument()
    expect(screen.getByText('Retry')).toBeInTheDocument()
  })

  it('retry resets error state', () => {
    const { rerender } = render(
      <ErrorBoundary name="test">
        <ThrowComponent shouldThrow />
      </ErrorBoundary>
    )
    expect(screen.getByText('页面出了点问题')).toBeInTheDocument()

    // Click retry
    screen.getByText('重试').click()
    // Re-render without throwing
    rerender(
      <ErrorBoundary name="test">
        <ThrowComponent shouldThrow={false} />
      </ErrorBoundary>
    )
    expect(screen.getByText('OK')).toBeInTheDocument()
  })

  it('calls onError callback', () => {
    const onError = vi.fn()
    render(
      <ErrorBoundary name="test" onError={onError}>
        <ThrowComponent shouldThrow />
      </ErrorBoundary>
    )
    expect(onError).toHaveBeenCalledTimes(1)
    expect(onError.mock.calls[0][0]).toBeInstanceOf(Error)
  })
})
