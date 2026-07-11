import type { ScreenId } from '../data/types'
import { FEATURE_FLAGS } from '../featureFlags'
import type { FeatureId } from '../../../progression'
import { useI18n } from '../../../i18n'

interface BottomTabBarProps {
  active: ScreenId
  onChange: (screen: ScreenId) => void
  onAchievement?: () => void
  /** When provided, locked features are omitted (no toast spam) */
  unlockedFeatures?: Set<FeatureId>
}

const tabs: { id: ScreenId | 'achievement'; feature: FeatureId; labelKey: string; icon: string }[] = [
  { id: 'discover', feature: 'discover', labelKey: 'tab.camera', icon: '◎' },
  { id: 'pokedex', feature: 'pokedex', labelKey: 'tab.collection', icon: '▣' },
  { id: 'battle', feature: 'battle', labelKey: 'tab.fight', icon: '✦' },
  { id: 'store', feature: 'store', labelKey: 'tab.store', icon: '◇' },
  { id: 'achievement', feature: 'achievement', labelKey: 'tab.achievement', icon: '☆' },
]

export default function BottomTabBar({
  active,
  onChange,
  onAchievement,
  unlockedFeatures,
}: BottomTabBarProps) {
  const { t } = useI18n()
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
              {t(tab.labelKey)}
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
            {t(tab.labelKey)}
          </button>
        )
      })}
    </nav>
  )
}
