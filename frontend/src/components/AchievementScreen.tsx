import React, { useState, useMemo } from 'react'
import { useAchievement } from '../achievement/useAchievement'
import { useStamina } from '../stamina/useStamina'
import { useShop } from '../shop/useShop'
import {
  CATEGORY_LABELS,
  ACHIEVEMENT_RARITY_COLORS,
  RARITY_LABELS,
} from '../achievement/constants'
import type { AchievementCategory, AchievementStats } from '../achievement/types'
import type { RarityTier, SpeciesType } from '../types'
import type { WeatherType } from '../weather/types'
import { getCompletionRate } from '../achievement/logic'

const CATEGORIES: AchievementCategory[] = ['collection', 'battle', 'exploration', 'social', 'milestone']

const AchievementScreen: React.FC = () => {
  const achievement = useAchievement()
  const stamina = useStamina()
  const shop = useShop()
  const [activeCategory, setActiveCategory] = useState<AchievementCategory | 'all'>('all')

  // 构建统计数据（简化版，使用 StaminaState 中已有的字段）
  const stats: AchievementStats = useMemo(() => {
    return {
      totalCaptures: stamina.state.totalCaptures,
      totalBattlesWon: stamina.state.totalBattlesWon,
      totalBattles: stamina.state.totalBattles,
      currentWinStreak: stamina.state.currentWinStreak,
      maxWinStreak: stamina.state.maxWinStreak,
      level: stamina.state.level,
      checkInStreak: shop.state.checkIn.streak,
      citiesVisited: 0,
      weatherTypesExperienced: [] as WeatherType[],
      capturesByRarity: {
        common: 0, uncommon: 0, rare: 0, epic: 0, legendary: 0,
      } as Record<RarityTier, number>,
      capturesBySpecies: {
        cat: 0, goose: 0, dog: 0,
      } as Record<SpeciesType, number>,
      hasLegendary: false,
      rainCapturesNoCold: 0,
    }
  }, [stamina.state, shop.state.checkIn.streak])

  const allProgress = useMemo(() => {
    return achievement.getAllProgress(stats)
  }, [achievement, stats])

  const filteredProgress = useMemo(() => {
    if (activeCategory === 'all') return allProgress
    return allProgress.filter((p) => {
      const def = achievement.definitions.find((d) => d.id === p.id)
      return def?.category === activeCategory
    })
  }, [allProgress, activeCategory, achievement.definitions])

  const unlockedCount = achievement.getUnlockedCount()
  const totalCount = achievement.getTotalCount()
  const completionRate = getCompletionRate(unlockedCount)

  return (
    <div style={styles.container}>
      {/* 头部统计 */}
      <div style={styles.header}>
        <div style={styles.headerTitle}>成就</div>
        <div style={styles.headerStats}>
          <span style={styles.headerCount}>{unlockedCount}/{totalCount}</span>
          <span style={styles.headerRate}>完成率 {completionRate}%</span>
        </div>
        <div style={styles.progressOuter}>
          <div style={{ ...styles.progressInner, width: `${completionRate}%` }} />
        </div>
      </div>

      {/* 分类标签 */}
      <div style={styles.tabBar}>
        <button
          style={{ ...styles.tab, ...(activeCategory === 'all' ? styles.tabActive : {}) }}
          onClick={() => setActiveCategory('all')}
        >
          全部
        </button>
        {CATEGORIES.map((cat) => (
          <button
            key={cat}
            style={{ ...styles.tab, ...(activeCategory === cat ? styles.tabActive : {}) }}
            onClick={() => setActiveCategory(cat)}
          >
            {CATEGORY_LABELS[cat]}
          </button>
        ))}
      </div>

      {/* 成就列表 */}
      <div style={styles.list}>
        {filteredProgress.map((progress) => {
          const def = achievement.definitions.find((d) => d.id === progress.id)
          if (!def) return null
          const rarityColor = ACHIEVEMENT_RARITY_COLORS[def.rarity]

          return (
            <div
              key={progress.id}
              style={{
                ...styles.achievementItem,
                borderLeftColor: rarityColor,
                opacity: progress.unlocked ? 1 : 0.6,
              }}
            >
              <div style={styles.achievementIcon}>{def.icon}</div>
              <div style={styles.achievementBody}>
                <div style={styles.achievementHeader}>
                  <span style={styles.achievementName}>{def.name}</span>
                  <span style={{ ...styles.achievementRarity, color: rarityColor }}>
                    {RARITY_LABELS[def.rarity]}
                  </span>
                  {progress.unlocked && <span style={styles.unlockedBadge}>✅</span>}
                </div>
                <div style={styles.achievementDesc}>{def.description}</div>
                {!progress.unlocked && progress.target > 0 && (
                  <div style={styles.progressRow}>
                    <div style={styles.miniProgressOuter}>
                      <div
                        style={{
                          ...styles.miniProgressInner,
                          width: `${progress.percent}%`,
                          background: rarityColor,
                        }}
                      />
                    </div>
                    <span style={styles.progressText}>
                      {progress.current}/{progress.target}
                    </span>
                  </div>
                )}
                <div style={styles.rewardText}>
                  奖励: {def.reward.gold} 金币
                  {def.reward.title && ` + 称号「${def.reward.title}」`}
                </div>
              </div>
            </div>
          )
        })}
      </div>

      {/* 称号选择器 */}
      {achievement.state.unlockedTitles.length > 0 && (
        <div style={styles.titleSection}>
          <div style={styles.titleLabel}>当前称号:</div>
          <select
            style={styles.titleSelect}
            value={achievement.state.activeTitle ?? ''}
            onChange={(e) => achievement.setActiveTitle(e.target.value || null)}
          >
            <option value="">无</option>
            {achievement.state.unlockedTitles.map((title) => (
              <option key={title} value={title}>{title}</option>
            ))}
          </select>
        </div>
      )}
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    display: 'flex',
    flexDirection: 'column',
    height: '100%',
    overflow: 'hidden',
    background: 'var(--bg-main)',
  },
  header: {
    padding: '12px 16px',
    background: 'var(--orange-dark)',
    color: 'var(--white)',
    flexShrink: 0,
  },
  headerTitle: {
    fontSize: 18,
    fontWeight: 700,
  },
  headerStats: {
    display: 'flex',
    justifyContent: 'space-between',
    marginTop: 4,
    fontSize: 13,
  },
  headerCount: {
    fontWeight: 600,
  },
  headerRate: {
    opacity: 0.8,
  },
  progressOuter: {
    height: 6,
    borderRadius: 3,
    background: 'rgba(255,255,255,0.3)',
    marginTop: 6,
    overflow: 'hidden',
  },
  progressInner: {
    height: '100%',
    borderRadius: 3,
    background: 'var(--white)',
    transition: 'width 0.3s ease',
  },
  tabBar: {
    display: 'flex',
    gap: 4,
    padding: '8px 12px',
    overflowX: 'auto',
    flexShrink: 0,
    borderBottom: '1px solid var(--orange-50)',
  },
  tab: {
    padding: '6px 12px',
    borderRadius: 16,
    border: 'none',
    background: 'var(--white)',
    color: 'var(--ink-3)',
    fontSize: 12,
    fontWeight: 600,
    cursor: 'pointer',
    whiteSpace: 'nowrap',
    fontFamily: 'inherit',
  },
  tabActive: {
    background: 'var(--orange)',
    color: 'var(--white)',
  },
  list: {
    flex: 1,
    overflowY: 'auto',
    padding: '8px 12px',
  },
  achievementItem: {
    display: 'flex',
    gap: 10,
    padding: 10,
    marginBottom: 8,
    borderRadius: 10,
    background: 'var(--white)',
    borderLeft: '3px solid var(--ink-4)',
  },
  achievementIcon: {
    fontSize: 28,
    lineHeight: 1,
    flexShrink: 0,
  },
  achievementBody: {
    flex: 1,
    minWidth: 0,
  },
  achievementHeader: {
    display: 'flex',
    alignItems: 'center',
    gap: 6,
  },
  achievementName: {
    fontSize: 14,
    fontWeight: 700,
    color: 'var(--ink-1)',
  },
  achievementRarity: {
    fontSize: 11,
    fontWeight: 600,
  },
  unlockedBadge: {
    marginLeft: 'auto',
    fontSize: 14,
  },
  achievementDesc: {
    fontSize: 12,
    color: 'var(--ink-3)',
    marginTop: 2,
  },
  progressRow: {
    display: 'flex',
    alignItems: 'center',
    gap: 6,
    marginTop: 6,
  },
  miniProgressOuter: {
    flex: 1,
    height: 5,
    borderRadius: 3,
    background: 'var(--orange-50)',
    overflow: 'hidden',
  },
  miniProgressInner: {
    height: '100%',
    borderRadius: 3,
    transition: 'width 0.3s ease',
  },
  progressText: {
    fontSize: 11,
    color: 'var(--ink-3)',
    whiteSpace: 'nowrap',
  },
  rewardText: {
    fontSize: 11,
    color: 'var(--ink-4)',
    marginTop: 4,
  },
  titleSection: {
    display: 'flex',
    alignItems: 'center',
    gap: 8,
    padding: '10px 16px',
    borderTop: '1px solid var(--orange-50)',
    flexShrink: 0,
  },
  titleLabel: {
    fontSize: 13,
    fontWeight: 600,
    color: 'var(--ink-2)',
  },
  titleSelect: {
    flex: 1,
    padding: '6px 10px',
    borderRadius: 8,
    border: '1px solid var(--orange-50)',
    background: 'var(--white)',
    fontSize: 13,
    fontFamily: 'inherit',
    color: 'var(--ink-1)',
  },
}

export default AchievementScreen
