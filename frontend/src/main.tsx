import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import { loadPublicConfig } from './config/publicConfig'
import { ensureAuth } from './auth/deviceAuth'
import { flushSyncQueue, installSyncOnlineFlush } from './services/syncQueue'
import { pullAnimalsFromServer } from './services/syncPull'
import { GlobalErrorBoundary, setupGlobalErrorHandlers, installOnlineListener } from './errors'
import { onNeedRefresh, applyPendingUpdate, getUpdateGateState } from './pwa/updateGate'
import './index.css'
import './a11y/a11y.css'

setupGlobalErrorHandlers()
installOnlineListener()

async function bootstrap() {
  // AP-040: controlled SW update (prompt). Defer while capture is active via updateGate.
  try {
    const { registerSW } = await import('virtual:pwa-register')
    const updateSW = registerSW({
      immediate: true,
      onNeedRefresh() {
        onNeedRefresh(() => updateSW(true))
        const st = getUpdateGateState()
        if (st.needRefresh && !st.capturing && !st.deferred) {
          // Non-blocking: expose for UI; auto-apply only if capture not active after short delay is avoided.
          // Operators/tests can call applyPendingUpdate().
          console.info('[pwa] update available — waiting for user or capture end')
        }
      },
      onOfflineReady() {
        console.info('[pwa] offline ready')
      },
    })
    // Export for debugging / future banner
    ;(window as unknown as { __AP_APPLY_UPDATE__?: () => Promise<boolean> }).__AP_APPLY_UPDATE__ = applyPendingUpdate
  } catch {
    // virtual:pwa-register unavailable in some test environments
  }

  try {
    loadPublicConfig()
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err)
    const rootEl = document.getElementById('root')
    if (rootEl) {
      rootEl.innerHTML =
        '<pre style="padding:16px;color:#4A2C1A;background:#FFF8F0;white-space:pre-wrap;font-family:ui-monospace,monospace">' +
        'Config error: ' + message + '

See frontend/.env.example</pre>'
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
  void (async () => {
    try {
      await pullAnimalsFromServer()
    } catch (e) {
      console.warn('sync pull deferred', e)
    }
    await flushSyncQueue()
  })()

  ReactDOM.createRoot(document.getElementById('root')!).render(
    <React.StrictMode>
      <GlobalErrorBoundary>
        <App />
      </GlobalErrorBoundary>
    </React.StrictMode>,
  )
}

void bootstrap()
