import React from 'react'

/** 加载占位屏（暖色橙风格） */
const LoadingScreen: React.FC = () => {
  return (
    <div style={styles.container}>
      <div style={styles.spinner} />
      <p style={styles.text}>加载中...</p>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    flex: 1,
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    justifyContent: 'center',
    gap: 16,
    background: 'var(--cream)',
  },
  spinner: {
    width: 40,
    height: 40,
    borderRadius: '50%',
    border: '4px solid var(--orange-100)',
    borderTopColor: 'var(--orange)',
    animation: 'spin 0.8s linear infinite',
  },
  text: {
    fontSize: 14,
    fontWeight: 600,
    color: 'var(--ink-2)',
  },
}

export default LoadingScreen
