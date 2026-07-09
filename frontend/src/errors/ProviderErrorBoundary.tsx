import React from 'react'
import { ErrorBoundary } from './ErrorBoundary'

interface Props {
  providerName: string
  children: React.ReactNode
}

export const ProviderErrorBoundary: React.FC<Props> = ({ providerName, children }) => {
  return (
    <ErrorBoundary
      name={providerName}
      fallback={() => (
        <div style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          minHeight: 60,
          color: 'var(--ink-3, #8B6F5C)',
          fontSize: 11,
        }}>
          ⚠️ {providerName} 模块暂时不可用
        </div>
      )}
    >
      {children}
    </ErrorBoundary>
  )
}
