import React from 'react'
import { ErrorBoundary } from './ErrorBoundary'

export const GlobalErrorBoundary: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  return (
    <ErrorBoundary
      name="global"
      fallback={() => (
        <div style={{
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          height: '100vh',
          padding: 24,
          background: 'var(--cream, #FFF8F0)',
        }}>
          <span style={{ fontSize: 56 }}>💥</span>
          <h2 style={{ color: 'var(--orange-dark, #E67300)', margin: '16px 0 8px', fontSize: 18 }}>
            应用崩溃了
          </h2>
          <p style={{ color: 'var(--ink-3, #8B6F5C)', fontSize: 13, textAlign: 'center', marginBottom: 20 }}>
            抱歉，遇到了严重错误。请尝试重新加载。
          </p>
          <button
            onClick={() => window.location.reload()}
            style={{ padding: '10px 32px', fontSize: 14, borderRadius: 10, border: 'none', background: 'var(--orange, #FF8C42)', color: '#fff', cursor: 'pointer' }}
          >
            重新加载
          </button>
        </div>
      )}
    >
      {children}
    </ErrorBoundary>
  )
}
