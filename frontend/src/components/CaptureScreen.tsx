import React, { useState, useRef, useCallback, useEffect } from 'react'
import type { CardEntry, RarityTier, SpeciesType } from '../types'
import { SPECIES_DEFS, SPECIES_RARITY_WEIGHTS } from '../types'
import { useStamina } from '../stamina/useStamina'
import { useShop } from '../shop/useShop'

// 投掷力度相关常量
const TICK_MS = 50          // 充能刷新间隔 50ms
const BASE_SUCCESS_RATE = 0.70  // 基础命中率

/** 投掷状态机 */
type ThrowState = 'idle' | 'charging' | 'throwing' | 'success' | 'fail'

export interface CaptureScreenProps {
  /** 当前目标物种 */
  targetSpecies?: SpeciesType
  onCaptureSuccess?: (entry: CardEntry) => void
  onCaptureFail?: () => void
}

/** 根据物种权重随机稀有度 */
function randomRarity(weights: { tier: RarityTier; weight: number }[]): RarityTier {
  const total = weights.reduce((s, r) => s + r.weight, 0)
  let roll = Math.random() * total
  for (const { tier, weight } of weights) {
    roll -= weight
    if (roll <= 0) return tier
  }
  return 'common'
}

/** 生成随机 CardEntry（含物种） */
function generateCardEntry(species: SpeciesType): CardEntry {
  const weights = SPECIES_RARITY_WEIGHTS[species]
  return {
    id: `c_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`,
    no: `#${String(Math.floor(Math.random() * 60 + 1)).padStart(6, '0')}`,
    rarity: randomRarity(weights),
    species,
    unlocked: true,
    captureDate: new Date().toISOString().split('T')[0],
    location: '宁波·未知区域',
    lat: 29.87 + (Math.random() - 0.5) * 0.05,
    lng: 121.55 + (Math.random() - 0.5) * 0.05,
    seed: Math.floor(Math.random() * 1000),
    isNew: true,
  }
}

const CaptureScreen: React.FC<CaptureScreenProps> = ({
  targetSpecies = 'cat',
  onCaptureSuccess,
  onCaptureFail,
}) => {
  const stamina = useStamina()
  const shop = useShop()

  // 从 SPECIES_DEFS 读取当前物种参数
  const def = SPECIES_DEFS[targetSpecies]
  const chargeRate = def.chargeRate
  const [optimalMin, optimalMax] = def.optimalRange

  const [throwState, setThrowState] = useState<ThrowState>('idle')
  const [charge, setCharge] = useState(0)
  const chargeRef = useRef(0)
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const canThrow = stamina.state.currentStamina >= 20
  const boostPercent = shop.getCaptureBoost()
  const baseRatePercent = Math.round(BASE_SUCCESS_RATE * 100)
  const boostedRatePercent = Math.min(baseRatePercent + boostPercent, 100)

  // 清理定时器
  const clearTimers = useCallback(() => {
    if (intervalRef.current) {
      clearInterval(intervalRef.current)
      intervalRef.current = null
    }
    if (timerRef.current) {
      clearTimeout(timerRef.current)
      timerRef.current = null
    }
  }, [])

  // 组件卸载时清理
  useEffect(() => {
    return () => clearTimers()
  }, [clearTimers])

  // 根据充能百分比计算命中率
  const calcSuccessRate = useCallback((charge: number): number => {
    if (charge >= optimalMin && charge <= optimalMax) {
      return BASE_SUCCESS_RATE
    }
    if (charge < optimalMin) {
      return BASE_SUCCESS_RATE * (charge / optimalMin)
    }
    // charge > optimalMax
    return BASE_SUCCESS_RATE * ((100 - charge) / (100 - optimalMax))
  }, [optimalMin, optimalMax])

  // 开始充能（按住按钮）
  const startCharging = useCallback(() => {
    if (!canThrow || throwState !== 'idle') return
    setThrowState('charging')
    chargeRef.current = 0
    setCharge(0)
    intervalRef.current = setInterval(() => {
      chargeRef.current = Math.min(chargeRef.current + chargeRate, 100)
      setCharge(chargeRef.current)
    }, TICK_MS)
  }, [canThrow, throwState, chargeRate])

  // 投掷（松手）
  const throwBall = useCallback(() => {
    if (throwState !== 'charging') return
    clearTimers()

    // 消耗体力
    stamina.consumeStamina(20)

    const finalCharge = chargeRef.current
    const successRate = calcSuccessRate(finalCharge)
    // 应用玩具球捕获增益
    const currentBoost = shop.getCaptureBoost()
    const boostedRate = Math.min(successRate + currentBoost / 100, 1)
    const isSuccess = Math.random() < boostedRate

    // 消耗捕获增益（投掷后即消耗）
    if (currentBoost > 0) {
      shop.consumeCaptureBoost()
    }

    setThrowState('throwing')

    timerRef.current = setTimeout(() => {
      if (isSuccess) {
        setThrowState('success')
        const entry = generateCardEntry(targetSpecies)
        onCaptureSuccess?.(entry)
      } else {
        setThrowState('fail')
        onCaptureFail?.()
      }
    }, 600)
  }, [throwState, clearTimers, stamina, shop, calcSuccessRate, onCaptureSuccess, onCaptureFail, targetSpecies])

  // 回到待机状态
  const resetToIdle = useCallback(() => {
    setThrowState('idle')
    setCharge(0)
    chargeRef.current = 0
  }, [])

  // 处理按钮按下事件（支持触屏和鼠标）
  const handlePointerDown = useCallback((e: React.PointerEvent) => {
    e.preventDefault()
    startCharging()
  }, [startCharging])

  const handlePointerUp = useCallback((e: React.PointerEvent) => {
    e.preventDefault()
    throwBall()
  }, [throwBall])

  // 力度条颜色（最佳区间为绿色）
  const getPowerBarColor = () => {
    if (throwState === 'charging') {
      if (charge >= optimalMin && charge <= optimalMax) return 'var(--success)'
      if (charge < 20) return 'var(--warn)'
      return 'var(--orange)'
    }
    return 'var(--orange)'
  }

  // ---- 投掷中动画 ----
  if (throwState === 'throwing') {
    return (
      <div style={styles.container}>
        <div style={styles.sky} />
        <div style={styles.ground} />
        <div style={styles.target}>{def.emoji}</div>
        <div style={styles.throwAnim}>
          {def.throwItemEmoji}
        </div>
        <div style={styles.centerMessage}>投掷中…</div>
      </div>
    )
  }

  // ---- 成功状态 ----
  if (throwState === 'success') {
    return (
      <div style={styles.container}>
        <div style={styles.sky} />
        <div style={styles.ground} />
        <div style={styles.target}>{def.emoji}</div>
        <div style={styles.centerMessage}>
          <span style={{ fontSize: 48, display: 'block', marginBottom: 8 }}>🎉</span>
          捕获成功！
        </div>
        <button
          className="btn btn-primary"
          style={styles.actionBtn}
          onClick={resetToIdle}
        >
          继续捕获
        </button>
      </div>
    )
  }

  // ---- 失败状态 ----
  if (throwState === 'fail') {
    return (
      <div style={styles.container}>
        <div style={styles.sky} />
        <div style={styles.ground} />
        <div style={styles.target}>{def.emoji}</div>
        <div style={styles.centerMessage}>
          <span style={{ fontSize: 48, display: 'block', marginBottom: 8 }}>😿</span>
          差一点！
        </div>
        <button
          className="btn btn-primary"
          style={styles.actionBtn}
          onClick={resetToIdle}
        >
          再试一次
        </button>
      </div>
    )
  }

  // ---- 待机 + 充能状态 ----
  return (
    <div style={styles.container}>
      {/* Sky + Ground */}
      <div style={styles.sky} />
      <div style={styles.ground} />

      {/* Top info pills */}
      <div style={styles.topInfo}>
        <span className="pill" style={styles.pill}>🎯 警惕值 中</span>
        {boostPercent > 0 ? (
          <>
            <span className="pill" style={{ ...styles.pill, background: 'var(--orange)' }}>
              🎾 +{boostPercent}%
            </span>
            <span className="pill" style={{ ...styles.pill, background: 'var(--success)' }}>
              {baseRatePercent}% → {boostedRatePercent}%
            </span>
          </>
        ) : (
          <span className="pill" style={{ ...styles.pill, background: 'var(--success)' }}>
            基础 {baseRatePercent}%
          </span>
        )}
        <span className="pill" style={styles.pill}>{def.throwItemEmoji} ×8</span>
      </div>

      {/* Instruction */}
      <span style={styles.instruction}>
        {throwState === 'idle' ? `按住投掷 · 松手命中 (${def.captureMechanics})` : '力度条充能中…'}
      </span>

      {/* Target animal emoji */}
      <div style={styles.target}>{def.emoji}</div>

      {/* Arc line (decorative) */}
      <svg style={styles.svgOverlay} viewBox="0 0 100 100" preserveAspectRatio="none">
        <path d="M5,70 Q50,20 95,30" fill="none" stroke="var(--orange)" strokeWidth="1.5" strokeDasharray="4,3" opacity="0.6" />
      </svg>

      {/* Power bar */}
      <div style={styles.powerWrap}>
        <span style={{ color: 'var(--white)', fontSize: 11, fontWeight: 600, textShadow: '0 1px 2px rgba(0,0,0,0.6)' }}>
          力度 {charge}%
        </span>
        <div style={styles.powerTrack}>
          <div
            style={{
              ...styles.powerFill,
              width: `${charge}%`,
              background: getPowerBarColor(),
              transition: throwState === 'idle' ? 'width 0.2s' : 'none',
            }}
          />
        </div>
        {/* 最佳区间标记 */}
        <div style={styles.optimalMarkers}>
          <div style={{ ...styles.optimalMarker, left: `${optimalMin}%` }} />
          <div style={{ ...styles.optimalMarker, left: `${optimalMax}%` }} />
        </div>
      </div>

      {/* Throw button */}
      {canThrow ? (
        <button
          className={`btn btn-primary ${throwState === 'charging' ? '' : ''}`}
          style={{
            ...styles.throwBtn,
            cursor: 'pointer',
          }}
          onPointerDown={handlePointerDown}
          onPointerUp={handlePointerUp}
          onPointerLeave={handlePointerUp}
        >
          {def.throwItemEmoji} 投掷
        </button>
      ) : (
        <div style={{ ...styles.throwBtn, ...styles.disabledBtn, cursor: 'not-allowed' }}>
          ⚡ 体力不足
        </div>
      )}
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    flex: 1,
    position: 'relative',
    overflow: 'hidden',
    background: '#6CAD6C',
  },
  sky: {
    position: 'absolute',
    top: 0,
    left: 0,
    right: 0,
    height: '60%',
    background: '#88D0EC',
  },
  ground: {
    position: 'absolute',
    bottom: 0,
    left: 0,
    right: 0,
    height: '45%',
    background: 'linear-gradient(180deg, #7BB06B, #5A9048)',
  },
  topInfo: {
    position: 'absolute',
    top: 8,
    left: 12,
    right: 12,
    display: 'flex',
    gap: 6,
    zIndex: 2,
  },
  pill: {
    fontSize: 10,
    padding: '3px 8px',
  },
  instruction: {
    position: 'absolute',
    top: 48,
    left: '50%',
    transform: 'translateX(-50%)',
    color: 'var(--white)',
    fontSize: 11,
    fontWeight: 600,
    textShadow: '0 1px 2px rgba(0,0,0,0.6)',
    zIndex: 1,
  },
  target: {
    position: 'absolute',
    top: '18%',
    left: '50%',
    transform: 'translateX(-50%)',
    fontSize: 64,
    zIndex: 1,
  },
  svgOverlay: {
    position: 'absolute',
    inset: 0,
    width: '100%',
    height: '100%',
    pointerEvents: 'none',
    zIndex: 1,
  },
  powerWrap: {
    position: 'absolute',
    bottom: 80,
    left: 20,
    right: 20,
    display: 'flex',
    flexDirection: 'column',
    gap: 4,
    zIndex: 2,
  },
  powerTrack: {
    width: '100%',
    height: 12,
    borderRadius: 6,
    background: 'rgba(255,255,255,0.4)',
    border: '2px solid rgba(255,255,255,0.8)',
    overflow: 'hidden',
    position: 'relative',
  },
  powerFill: {
    height: '100%',
    borderRadius: 6,
    background: 'var(--orange)',
  },
  optimalMarkers: {
    position: 'absolute',
    top: 28,
    left: 0,
    right: 0,
    height: 4,
    pointerEvents: 'none',
  },
  optimalMarker: {
    position: 'absolute',
    width: 2,
    height: 10,
    background: 'var(--success)',
    top: -5,
    transform: 'translateX(-50%)',
  },
  throwBtn: {
    position: 'absolute',
    bottom: 12,
    left: '50%',
    transform: 'translateX(-50%)',
    padding: '8px 28px',
    fontSize: 16,
    borderRadius: 24,
    zIndex: 2,
    userSelect: 'none',
    touchAction: 'none',
  },
  disabledBtn: {
    position: 'absolute',
    bottom: 12,
    left: '50%',
    transform: 'translateX(-50%)',
    padding: '8px 28px',
    fontSize: 16,
    borderRadius: 24,
    zIndex: 2,
    background: 'var(--ink-3)',
    color: 'var(--white)',
    opacity: 0.7,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
  },
  centerMessage: {
    position: 'absolute',
    top: '35%',
    left: '50%',
    transform: 'translate(-50%, -50%)',
    color: 'var(--white)',
    fontSize: 22,
    fontWeight: 700,
    textShadow: '0 2px 4px rgba(0,0,0,0.5)',
    zIndex: 3,
    textAlign: 'center',
  },
  actionBtn: {
    position: 'absolute',
    bottom: 24,
    left: '50%',
    transform: 'translateX(-50%)',
    padding: '8px 28px',
    fontSize: 16,
    borderRadius: 24,
    zIndex: 3,
  },
  throwAnim: {
    position: 'absolute',
    top: '25%',
    left: '50%',
    transform: 'translateX(-50%)',
    fontSize: 36,
    zIndex: 3,
    animation: 'none',
  },
}

export default React.memo(CaptureScreen)
