export type {
  FeatureId,
  GoalDef,
  GoalHorizon,
  GoalProgress,
  ProgressSnapshot,
  ProgressionState,
  ReturnSummary,
  ProgressionContextValue,
} from './types'
export {
  GOAL_DEFS,
  GOAL_MAP,
  FEATURE_GATES,
  PROGRESSION_STORAGE_KEY,
  EXECUTABLE_GOAL_COUNT,
} from './constants'
export {
  selectExecutableGoals,
  buildReturnSummary,
  isFeatureUnlocked,
  getVisibleTabs,
  computeGoalProgress,
  createDefaultProgressionState,
  buildSnapshot,
} from './logic'
export { ProgressionProvider, useProgression, ProgressionContext } from './ProgressionContext'
