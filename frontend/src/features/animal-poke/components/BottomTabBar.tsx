import type { ScreenId } from '../data/types'
import { FEATURE_FLAGS } from '../featureFlags'
import type { FeatureId } from '../../../progression'

interface BottomTabBarProps {
  active: ScreenId
  onChange: (screen: ScreenId) => void
  onAchievement?: () => void
  /** When provided, locked features are omitted (no toast spam) */
  unlockedFeatures?: Set<FeatureId>
}

const tabs: { id: ScreenId | 'achievement'; feature: FeatureId; label: string; icon: string }[] = [
  { id: 'discover', feature: 'discover', label: '发现', icon: '◎' },
  { id: 'pokedex', feature: 'pokedex', label: '图鉴', icon: '▣' },
  { id: 'battle', feature: 'battle', label: '战斗', icon: '✦' },
  { id: 'store', feature: 'store', label: '商店', icon: '◇' },
  { id: 'achievement', feature: 'achievement', label: '成就', icon: '☆' },
]

export default function BottomTabBar({
  active,
  onChange,
  onAchievement,
  unlockedFeatures,
}: BottomTabBarProps) {
  const visible = tabs.filter((tab) => {
    if (tab.id === 'achievement' && !FEATURE_FLAGS.achievements) return false
    if (!unlockedFeatures) return true
    return unlockedFeatures.has(tab.feature)
  })

  return (
    <nav className="ap-bottom-tabs" aria-label="底部导航">
      {visible.map((tab) => {
        if (tab.id === 'achievement') {
          if (!onAchievement) return null
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
