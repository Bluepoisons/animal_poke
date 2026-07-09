import React, { Component, type ReactNode } from 'react'
import type { ErrorReport } from './types'
import { reportError } from './reporter'

declare const __APP_VERSION__: string

interface Props {
  children: ReactNode
  name: string
  fallback?: ReactNode | ((error: Error, retry: () => void) => ReactNode)
  report?: boolean
  onError?: (error: Error, info: React.ErrorInfo) => void
}

interface State {
  hasError: boolean
  error: Error | null
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false, error: null }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, info: React.ErrorInfo): void {
    if (this.props.report !== false) {
      const report: ErrorReport = {
        id: typeof crypto !== 'undefined' && crypto.randomUUID
          ? crypto.randomUUID()
          : `${Date.now()}-${Math.random()}`,
        type: 'react',
        name: error.name,
        message: error.message,
        stack: error.stack,
        timestamp: Date.now(),
        page: this.props.name,
        appVersion: typeof __APP_VERSION__ !== 'undefined' ? __APP_VERSION__ : '0.0.0',
        userAgent: typeof navigator !== 'undefined' ? navigator.userAgent : 'unknown',
        context: { componentStack: info.componentStack },
        online: typeof navigator !== 'undefined' ? navigator.onLine : true,
      }
      reportError(report).catch(() => {})
    }
    this.props.onError?.(error, info)
  }

  retry = (): void => {
    this.setState({ hasError: false, error: null })
  }

  render(): ReactNode {
    if (this.state.hasError) {
      if (this.props.fallback) {
        return typeof this.props.fallback === 'function'
          ? this.props.fallback(this.state.error!, this.retry)
          : this.props.fallback
      }
      return <DefaultFallback error={this.state.error!} onRetry={this.retry} />
    }
    return this.props.children
  }
}

function DefaultFallback({ error, onRetry }: { error: Error; onRetry: () => void }) {
  return (
    <div style={{
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      height: '100%',
      padding: 24,
      background: 'var(--cream, #FFF8F0)',
    }}>
      <span style={{ fontSize: 48 }}>😵</span>
      <h3 style={{ color: 'var(--orange-dark, #E67300)', margin: '12px 0 4px', fontSize: 16 }}>
        页面出了点问题
      </h3>
      <p style={{ color: 'var(--ink-3, #8B6F5C)', fontSize: 12, textAlign: 'center', marginBottom: 16 }}>
        {error.message || '发生了未知错误'}
      </p>
      <button onClick={onRetry} style={{ padding: '8px 24px', fontSize: 13, borderRadius: 10, border: 'none', background: 'var(--orange, #FF8C42)', color: '#fff', cursor: 'pointer' }}>
        重试
      </button>
    </div>
  )
}
