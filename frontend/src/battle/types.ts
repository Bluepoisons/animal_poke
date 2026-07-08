import type { RarityTier, SpeciesType, CardEntry } from '../types'

// ===== 元素类型 =====
export type ElementType = 'fire' | 'water' | 'grass' | 'light' | 'dark'

// ===== 策略类型 =====
export type StrategyType = 'aggressive' | 'balanced' | 'defensive'

// ===== 天气类型 =====
export type WeatherType = 'sunny' | 'cloudy' | 'overcast' | 'rainy' | 'snowy' | 'foggy' | 'extreme'

// ===== 战斗阶段 =====
export type BattlePhase = 'idle' | 'selecting' | 'matching' | 'battling' | 'result'

// ===== 战斗属性 =====
export interface BattleStats {
  hp: number
  atk: number
  def: number
  spd: number
  crit: number   // 暴击率 0~100
  eva: number    // 闪避率 0~100
}

// ===== 战斗宠物（运行时状态） =====
export interface BattlePet {
  id: string
  name: string
  emoji: string
  species: SpeciesType
  rarity: RarityTier
  element: ElementType
  stats: BattleStats         // 最终属性（含天气/策略修正）
  baseStats: BattleStats     // 原始属性（不含天气/策略修正）
  currentHp: number
  energy: number             // 0~100
  isPlayer: boolean
  strategy: StrategyType     // 仅玩家方使用
}

// ===== 日志条目 =====
export interface BattleLogEntry {
  round: number
  text: string
  type: 'attack' | 'crit' | 'miss' | 'ultimate' | 'item' | 'system'
}

// ===== 战斗结果 =====
export type BattleResult = 'win' | 'lose' | 'draw' | null

// ===== 战斗奖励 =====
export interface BattleRewards {
  gold: number
  exp: number
  droppedItem?: string
}

// ===== BattleState =====
export interface BattleState {
  phase: BattlePhase
  playerPet: BattlePet | null
  enemyPet: BattlePet | null
  round: number
  maxRounds: number
  log: BattleLogEntry[]
  result: BattleResult
  rewards: BattleRewards | null
  strategy: StrategyType
  weather: WeatherType
  isAutoPlaying: boolean
}

// ===== BattleAction =====
export type BattleAction =
  | { type: 'ENTER_SELECT' }
  | { type: 'SELECT_PET'; pet: BattlePet }
  | { type: 'START_MATCHING' }
  | { type: 'MATCH_COMPLETE'; enemy: BattlePet }
  | { type: 'BATTLE_START' }
  | { type: 'EXECUTE_ROUND'; log: BattleLogEntry[]; playerPet: BattlePet; enemyPet: BattlePet }
  | { type: 'USE_ULTIMATE'; log: BattleLogEntry[]; playerPet: BattlePet; enemyPet: BattlePet }
  | { type: 'SET_STRATEGY'; strategy: StrategyType }
  | { type: 'USE_ITEM'; log: BattleLogEntry[]; playerPet: BattlePet }
  | { type: 'BATTLE_END'; result: BattleResult; rewards: BattleRewards }
  | { type: 'RESET' }
  | { type: 'SET_WEATHER'; weather: WeatherType }
  | { type: 'SET_AUTO_PLAY'; playing: boolean }

// ===== BattleContextValue =====
export interface BattleContextValue {
  state: BattleState
  enterSelect: () => void
  selectPet: (entry: CardEntry) => boolean
  startMatching: () => void
  executeNextRound: () => void
  useUltimate: () => boolean
  setStrategy: (strategy: StrategyType) => void
  useBattleItem: (itemId: string) => boolean
  toggleAutoPlay: () => void
  finishBattle: () => void
  reset: () => void
}

export type { CardEntry } from '../types'
