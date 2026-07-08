import React from 'react'
import type { MainTab } from '../types'

const TABS: { key: MainTab; icon: string; label: string }[] = [
  { key: 'profile', icon: '👤', label: 'Profile' },
  { key: 'collection', icon: '📖', label: 'Collection' },
  { key: 'camera', icon: '📷', label: 'Camera' },
  { key: 'fight', icon: '🥊', label: 'Fight' },
  { key: 'store', icon: '🏪', label: 'Store' },
]

interface TabBarProps {
  activeTab: MainTab
  onTabChange: (tab: MainTab) => void
}

const TabBar: React.FC<TabBarProps> = ({ activeTab, onTabChange }) => {
  return (
    <div style={styles.container}>
      {TABS.map(tab => (
        <button
          key={tab.key}
          style={{
            ...styles.tab,
            ...(tab.key === activeTab ? styles.activeTab : {}),
            ...(tab.key === 'camera' ? styles.cameraTab : {}),
          }}
          onClick={() => onTabChange(tab.key)}
        >
          <span style={tab.key === 'camera' ? styles.cameraIcon : styles.tabIcon}>
            {tab.icon}
          </span>
          {tab.key !== 'camera' && <span style={styles.tabLabel}>{tab.label}</span>}
        </button>
      ))}
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    height: 60,
    background: 'var(--white)',
    borderTop: '2px solid var(--orange-50)',
    display: 'flex',
    justifyContent: 'space-around',
    alignItems: 'center',
    paddingBottom: 6,
    flexShrink: 0,
    zIndex: 2,
  },
  tab: {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    gap: 2,
    border: 'none',
    background: 'none',
    cursor: 'pointer',
    fontFamily: 'inherit',
    fontSize: 10,
    fontWeight: 600,
    color: 'var(--ink-3)',
    padding: '4px 6px',
    borderRadius: 8,
    transition: 'color 0.15s',
  },
  activeTab: {
    color: 'var(--orange-dark)',
    background: 'var(--orange-50)',
  },
  cameraTab: {
    width: 48,
    height: 48,
    borderRadius: '50%',
    background: 'var(--orange)',
    color: 'var(--white)',
    boxShadow: '0 4px 0 var(--orange-dark)',
    marginTop: -18,
    padding: 0,
    justifyContent: 'center',
  },
  tabIcon: {
    fontSize: 18,
    lineHeight: 1,
  },
  cameraIcon: {
    fontSize: 22,
    lineHeight: 1,
  },
  tabLabel: {
    fontSize: 10,
    lineHeight: 1,
  },
}

export default TabBar
