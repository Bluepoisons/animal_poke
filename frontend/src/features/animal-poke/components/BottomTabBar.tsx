import type { ScreenId } from '../data/types'
import { FEATURE_FLAGS } from '../featureFlags'
import { useI18n } from '../../../i18n'

interface BottomTabBarProps {
  active: ScreenId
  onChange: (screen: ScreenId) => void
  onAchievement?: () => void
}

export default function BottomTabBar({
  active,
  onChange,
  onAchievement,
}: BottomTabBarProps) {
  const { t } = useI18n()
  const tabs: { id: ScreenId | 'achievement'; label: string; icon: string }[] = [
    { id: 'discover', label: t('tab.camera'), icon: '◎' },
    { id: 'pokedex', label: t('tab.collection'), icon: '▣' },
    { id: 'battle', label: t('tab.fight'), icon: '✦' },
    { id: 'store', label: t('tab.store'), icon: '◇' },
    { id: 'settings', label: t('tab.settings'), icon: '⚙' },
  ]

  return (
    <nav className="ap-bottom-tabs" aria-label="bottom navigation">
      {tabs.filter((tab) => tab.id !== 'achievement' || FEATURE_FLAGS.achievements).map((tab) => {
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
