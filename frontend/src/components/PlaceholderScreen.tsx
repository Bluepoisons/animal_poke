import React from 'react'

interface PlaceholderProps {
  icon: string
  title: string
  subtitle: string
}

const PlaceholderScreen: React.FC<PlaceholderProps> = ({ icon, title, subtitle }) => (
  <div style={styles.container}>
    <div style={styles.card}>
      <span style={styles.icon}>{icon}</span>
      <h2 style={styles.title}>{title}</h2>
      <p style={styles.subtitle}>{subtitle}</p>
    </div>
  </div>
)

const styles: Record<string, React.CSSProperties> = {
  container: {
    flex: 1,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    background: 'var(--cream)',
    padding: 20,
  },
  card: {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    gap: 10,
    padding: '32px 24px',
    background: 'var(--white)',
    borderRadius: 20,
    boxShadow: 'var(--shadow-card)',
    maxWidth: 240,
    width: '100%',
  },
  icon: {
    fontSize: 48,
  },
  title: {
    fontSize: 20,
    fontWeight: 700,
    color: 'var(--orange-dark)',
    margin: 0,
  },
  subtitle: {
    fontSize: 13,
    color: 'var(--ink-3)',
    margin: 0,
    textAlign: 'center' as const,
  },
}

export default PlaceholderScreen
