import React from 'react'

const CaptureScreen: React.FC = () => (
  <div style={styles.container}>
    {/* Sky + Ground */}
    <div style={styles.sky} />
    <div style={styles.ground} />

    {/* Top info pills */}
    <div style={styles.topInfo}>
      <span className="pill" style={styles.pill}>🎯 警惕值 中</span>
      <span className="pill" style={{ ...styles.pill, background: 'var(--success)' }}>基础 70%</span>
      <span className="pill" style={styles.pill}>🥫 ×8</span>
    </div>

    {/* Instruction */}
    <span style={styles.instruction}>滑动瞄准 · 松手投掷</span>

    {/* Target animal emoji */}
    <div style={styles.target}>🐱</div>

    {/* Arc line (decorative) */}
    <svg style={styles.svgOverlay} viewBox="0 0 100 100" preserveAspectRatio="none">
      <path d="M5,70 Q50,20 95,30" fill="none" stroke="var(--orange)" strokeWidth="1.5" strokeDasharray="4,3" opacity="0.6" />
    </svg>

    {/* Power bar */}
    <div style={styles.powerWrap}>
      <span style={{ color: 'var(--white)', fontSize: 11, fontWeight: 600, textShadow: '0 1px 2px rgba(0,0,0,0.6)' }}>
        力度 65%
      </span>
      <div style={styles.powerTrack}>
        <div style={styles.powerFill} />
      </div>
    </div>

    {/* Throw button */}
    <button className="btn btn-primary" style={styles.throwBtn}>🥫 投掷</button>
  </div>
)

const styles: Record<string, React.CSSProperties> = {
  container: {
    flex: 1,
    position: 'relative',
    overflow: 'hidden',
    background: '#6CAD6C',
  },
  sky: {
    position: 'absolute',
    top: 0,
    left: 0,
    right: 0,
    height: '60%',
    background: '#88D0EC',
  },
  ground: {
    position: 'absolute',
    bottom: 0,
    left: 0,
    right: 0,
    height: '45%',
    background: 'linear-gradient(180deg, #7BB06B, #5A9048)',
  },
  topInfo: {
    position: 'absolute',
    top: 8,
    left: 12,
    right: 12,
    display: 'flex',
    gap: 6,
    zIndex: 2,
  },
  pill: {
    fontSize: 10,
    padding: '3px 8px',
  },
  instruction: {
    position: 'absolute',
    top: 48,
    left: '50%',
    transform: 'translateX(-50%)',
    color: 'var(--white)',
    fontSize: 11,
    fontWeight: 600,
    textShadow: '0 1px 2px rgba(0,0,0,0.6)',
    zIndex: 1,
  },
  target: {
    position: 'absolute',
    top: '18%',
    left: '50%',
    transform: 'translateX(-50%)',
    fontSize: 64,
    zIndex: 1,
  },
  svgOverlay: {
    position: 'absolute',
    inset: 0,
    width: '100%',
    height: '100%',
    pointerEvents: 'none',
    zIndex: 1,
  },
  powerWrap: {
    position: 'absolute',
    bottom: 80,
    left: 20,
    right: 20,
    display: 'flex',
    flexDirection: 'column',
    gap: 4,
    zIndex: 2,
  },
  powerTrack: {
    width: '100%',
    height: 12,
    borderRadius: 6,
    background: 'rgba(255,255,255,0.4)',
    border: '2px solid rgba(255,255,255,0.8)',
    overflow: 'hidden',
  },
  powerFill: {
    width: '65%',
    height: '100%',
    borderRadius: 6,
    background: 'var(--orange)',
  },
  throwBtn: {
    position: 'absolute',
    bottom: 12,
    left: '50%',
    transform: 'translateX(-50%)',
    padding: '8px 28px',
    fontSize: 16,
    borderRadius: 24,
    zIndex: 2,
  },
}

export default CaptureScreen
