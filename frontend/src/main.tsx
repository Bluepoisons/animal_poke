import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import { GlobalErrorBoundary, setupGlobalErrorHandlers, installOnlineListener } from './errors'
import './index.css'
import './a11y/a11y.css'

setupGlobalErrorHandlers()
installOnlineListener()

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <GlobalErrorBoundary>
      <App />
    </GlobalErrorBoundary>
  </React.StrictMode>,
)
