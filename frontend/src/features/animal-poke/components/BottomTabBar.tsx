import type { ScreenId } from '../data/types'

interface BottomTabBarProps {
  active: ScreenId
  onChange: (screen: ScreenId) => void
  onAchievement?: () => void
}

const tabs: { id: ScreenId; label: string }[] = [
  { id: 'discover', label: '发现' },
  { id: 'pokedex', label: '图鉴' },
  { id: 'battle', label: '战斗' },
  { id: 'store', label: '商店' },
]

export default function BottomTabBar({
  active,
  onChange,
  onAchievement,
}: BottomTabBarProps) {
  return (
    <nav className="ap-bottom-tabs" aria-label="底部导航">
      {tabs.map((tab) => (
        <button
          key={tab.id}
          className={active === tab.id ? 'is-active' : ''}
          onClick={() => onChange(tab.id)}
          type="button"
        >
          {tab.label}
        </button>
      ))}
      <button onClick={onAchievement} type="button">
        成就
      </button>
    </nav>
  )
}
