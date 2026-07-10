import { useOnlineStatus } from '../hooks/useOnlineStatus'

/** 在线优先提示：断网仅可浏览图鉴 */
export function OfflineBanner() {
  const online = useOnlineStatus()
  if (online) return null
  return (
    <div
      role="status"
      aria-live="polite"
      style={{
        position: 'fixed',
        top: 0,
        left: 0,
        right: 0,
        zIndex: 9999,
        background: '#4A2C1A',
        color: '#FFF8F0',
        padding: '8px 12px',
        textAlign: 'center',
        fontSize: 13,
      }}
    >
      当前离线：发现/捕获不可用，仅可浏览图鉴
    </div>
  )
}
