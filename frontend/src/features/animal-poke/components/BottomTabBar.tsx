import type { ScreenId } from '../data/types'

interface BottomTabBarProps {
  active: ScreenId
  onChange: (screen: ScreenId) => void
  onAchievement?: () => void
}

const tabs: { id: ScreenId | 'achievement'; label: string; icon: string }[] = [
  { id: 'discover', label: '发现', icon: '◎' },
  { id: 'pokedex', label: '图鉴', icon: '▣' },
  { id: 'battle', label: '战斗', icon: '✦' },
  { id: 'store', label: '商店', icon: '◇' },
  { id: 'achievement', label: '成就', icon: '☆' },
]

export default function BottomTabBar({
  active,
  onChange,
  onAchievement,
}: BottomTabBarProps) {
  return (
    <nav className="ap-bottom-tabs" aria-label="底部导航">
      {tabs.map((tab) => {
        if (tab.id === 'achievement') {
          return (
            <button key={tab.id} onClick={onAchievement} type="button">
              <span className="ap-tab-icon" aria-hidden="true">
                {tab.icon}
              </span>
              {tab.label}
            </button>
          )
        }

        return (
          <button
            key={tab.id}
            className={active === tab.id ? 'is-active' : ''}
            onClick={() => onChange(tab.id as ScreenId)}
            type="button"
            aria-current={active === tab.id ? 'page' : undefined}
          >
            <span className="ap-tab-icon" aria-hidden="true">
              {tab.icon}
            </span>
            {tab.label}
          </button>
        )
      })}
    </nav>
  )
}
