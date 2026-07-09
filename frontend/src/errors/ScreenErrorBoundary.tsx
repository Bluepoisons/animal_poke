import React from 'react'
import { ErrorBoundary } from './ErrorBoundary'

interface Props {
  screenName: string
  children: React.ReactNode
}

export const ScreenErrorBoundary: React.FC<Props> = ({ screenName, children }) => {
  return (
    <ErrorBoundary
      name={`screen:${screenName}`}
      fallback={(error, retry) => (
        <div style={{
          flex: 1,
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          background: 'var(--cream, #FFF8F0)',
          padding: 24,
        }}>
          <span style={{ fontSize: 40 }}>😵</span>
          <h3 style={{ color: 'var(--orange-dark, #E67300)', margin: '12px 0 4px', fontSize: 15 }}>
            {screenName} 出了点问题
          </h3>
          <p style={{ color: 'var(--ink-3, #8B6F5C)', fontSize: 11, textAlign: 'center', marginBottom: 16 }}>
            {error.message}
          </p>
          <button onClick={retry} style={{ padding: '8px 20px', fontSize: 13, borderRadius: 10, border: 'none', background: 'var(--orange, #FF8C42)', color: '#fff', cursor: 'pointer' }}>
            重试
          </button>
        </div>
      )}
    >
      {children}
    </ErrorBoundary>
  )
}
