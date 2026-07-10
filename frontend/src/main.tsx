import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import { loadPublicConfig } from './config/publicConfig'
import { ensureAuth } from './auth/deviceAuth'
import { flushSyncQueue, installSyncOnlineFlush } from './services/syncQueue'
import { GlobalErrorBoundary, setupGlobalErrorHandlers, installOnlineListener } from './errors'
import './index.css'
import './a11y/a11y.css'

setupGlobalErrorHandlers()
installOnlineListener()

async function bootstrap() {
  try {
    loadPublicConfig()
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err)
    const rootEl = document.getElementById('root')
    if (rootEl) {
      rootEl.innerHTML =
        '<pre style="padding:16px;color:#4A2C1A;background:#FFF8F0;white-space:pre-wrap;font-family:ui-monospace,monospace">' +
        'Config error: ' + message + '\n\nSee frontend/.env.example</pre>'
    }
    throw err
  }

  // 设备鉴权：失败不阻断 UI（离线/后端未起时可浏览本地内容）
  try {
    await ensureAuth()
  } catch (err) {
    console.warn('device auth deferred:', err)
  }

  installSyncOnlineFlush()
  void flushSyncQueue()

  ReactDOM.createRoot(document.getElementById('root')!).render(
    <React.StrictMode>
      <GlobalErrorBoundary>
        <App />
      </GlobalErrorBoundary>
    </React.StrictMode>,
  )
}

void bootstrap()
