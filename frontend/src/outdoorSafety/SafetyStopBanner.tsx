/** Banner: stop first before outdoor capture (AP-045) */
export interface SafetyStopBannerProps {
  messages: string[]
  /** Primary “stop first” reminder always shown when outdoor blocked or high speed */
  showStopFirst?: boolean
}

const STOP_FIRST = '请先停下再操作，避免边走边看屏幕'

export default function SafetyStopBanner({
  messages,
  showStopFirst = true,
}: SafetyStopBannerProps) {
  if (!showStopFirst && messages.length === 0) return null

  return (
    <div
      className="ap-safety-banner"
      role="alert"
      style={{
        margin: '8px 12px',
        padding: '10px 12px',
        borderRadius: 12,
        background: '#FFF3E0',
        border: '1px solid #FFB74D',
        color: '#5D4037',
        fontSize: 13,
        lineHeight: 1.4,
      }}
    >
      {showStopFirst && (
        <div style={{ fontWeight: 600, marginBottom: messages.length ? 4 : 0 }}>
          ⚠ {STOP_FIRST}
        </div>
      )}
      {messages.map((m) => (
        <div key={m}>{m}</div>
      ))}
    </div>
  )
}

export { STOP_FIRST }
