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

const tabs: { id: ScreenId | 'achievement'; feature: FeatureId | 'settings'; labelKey: string; icon: string; always?: boolean }[] = [
  { id: 'discover', feature: 'discover', labelKey: 'tab.camera', icon: '◎' },
  { id: 'pokedex', feature: 'pokedex', labelKey: 'tab.collection', icon: '▣' },
  { id: 'battle', feature: 'battle', labelKey: 'tab.fight', icon: '✦' },
  { id: 'store', feature: 'store', labelKey: 'tab.store', icon: '◇' },
  { id: 'achievement', feature: 'achievement', labelKey: 'tab.achievement', icon: '☆' },
  // AP-061: settings always reachable from any core surface (≤2 taps)
  { id: 'settings', feature: 'settings', labelKey: 'tab.settings', icon: '⚙', always: true },
]

export default function BottomTabBar({
  active,
  onChange,
  onAchievement,
  unlockedFeatures,
}: BottomTabBarProps) {
  const { t } = useI18n()
  const visible = tabs.filter((tab) => {
    if (tab.always) return true
    if (tab.id === 'achievement' && !FEATURE_FLAGS.achievements) return false
    if (!unlockedFeatures) return true
    return unlockedFeatures.has(tab.feature as FeatureId)
  })

  return (
    <nav className="ap-bottom-tabs" aria-label="底部导航" data-testid="bottom-tab-bar">
      {visible.map((tab) => {
        if (tab.id === 'achievement') {
          if (!onAchievement) return null
          return (
            <button key={tab.id} onClick={onAchievement} type="button" data-testid="tab-achievement">
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
            data-testid={`tab-${tab.id}`}
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
