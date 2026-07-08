import { useContext } from 'react'
import { AchievementContext } from './AchievementContext'
import type { AchievementContextValue } from './types'

export function useAchievement(): AchievementContextValue {
  const ctx = useContext(AchievementContext)
  if (!ctx) {
    throw new Error('useAchievement must be used within AchievementProvider')
  }
  return ctx
}
