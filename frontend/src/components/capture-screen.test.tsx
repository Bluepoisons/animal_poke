import React from 'react'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, act, fireEvent } from '@testing-library/react'
import CaptureScreen from './CaptureScreen'

// Mock useStamina
const mockConsumeStamina = vi.fn()
let mockStamina = 120

vi.mock('../stamina/useStamina', () => ({
  useStamina: () => ({
    state: { currentStamina: mockStamina },
    consumeStamina: mockConsumeStamina,
    addCapture: vi.fn(),
    addGold: vi.fn(),
    addStamina: vi.fn(),
    buyStaminaPotion: vi.fn(),
    maxStamina: 120,
    nextRecoverIn: 360,
  }),
}))

// Mock useShop（捕获增益相关）
const mockGetCaptureBoost = vi.fn(() => 0)
const mockConsumeCaptureBoost = vi.fn()

vi.mock('../shop/useShop', () => ({
  useShop: () => ({
    state: { inventory: {}, checkIn: { streak: 0, lastCheckInDate: '' }, dailyPurchases: {}, dailyPurchaseDate: '' },
    getCaptureBoost: mockGetCaptureBoost,
    consumeCaptureBoost: mockConsumeCaptureBoost,
    isCaptureBoostActive: vi.fn(() => false),
    buyItem: vi.fn(),
    useItem: vi.fn(),
    checkIn: vi.fn(),
    getItemCount: vi.fn(() => 0),
    getDailyPurchaseCount: vi.fn(() => 0),
  }),
}))

const TICK_MS = 50

describe('CaptureScreen', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    mockStamina = 120
    mockConsumeStamina.mockClear()
    mockGetCaptureBoost.mockClear()
    mockGetCaptureBoost.mockReturnValue(0)
    mockConsumeCaptureBoost.mockClear()
    mockConsumeStamina.mockImplementation((amount: number) => {
      if (mockStamina >= amount) {
        return true
      }
      return false
    })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('力度条充能：按住按钮时 charge 从 0 开始上升', () => {
    render(<CaptureScreen />)

    const btn = screen.getByRole('button', { name: /投掷/ })
    expect(btn).toBeInTheDocument()

    // 初始 charge 0%
    expect(screen.getByText('力度 0%')).toBeInTheDocument()

    // 按下按钮开始充能
    fireEvent.pointerDown(btn)

    // 推进 500ms → charge = 500/50 * 2 = 20%
    act(() => {
      vi.advanceTimersByTime(500)
    })
    expect(screen.getByText('力度 20%')).toBeInTheDocument()

    // 再推进 1000ms → charge = 60%
    act(() => {
      vi.advanceTimersByTime(1000)
    })
    expect(screen.getByText('力度 60%')).toBeInTheDocument()
  })

  it('投掷成功后显示捕获成功提示', () => {
    const onSuccess = vi.fn()

    // mock Math.random 让 charge 落在最佳区间 + 判定成功
    vi.spyOn(Math, 'random').mockReturnValue(0.5)

    render(<CaptureScreen onCaptureSuccess={onSuccess} />)

    const btn = screen.getByRole('button', { name: /投掷/ })

    // 按住充能到 50%
    fireEvent.pointerDown(btn)
    act(() => {
      vi.advanceTimersByTime(1250) // 25 ticks * 2 = 50%
    })

    // 松手投掷
    fireEvent.pointerUp(btn)

    // consumeStamina 应被调用
    expect(mockConsumeStamina).toHaveBeenCalledWith(20)

    // 等待投掷动画完成（600ms）
    act(() => {
      vi.advanceTimersByTime(600)
    })

    // 应显示成功消息
    expect(screen.getByText('捕获成功！')).toBeInTheDocument()
    expect(onSuccess).toHaveBeenCalledOnce()
    const entryArg = onSuccess.mock.calls[0][0]
    expect(entryArg).toHaveProperty('id')
    expect(entryArg).toHaveProperty('no')
    expect(entryArg).toHaveProperty('rarity')
    expect(entryArg).toHaveProperty('captureDate')
    expect(entryArg.unlocked).toBe(true)
    expect(entryArg.isNew).toBe(true)

    vi.spyOn(Math, 'random').mockRestore()
  })

  it('投掷失败后显示差一点提示', () => {
    const onFail = vi.fn()

    // mock Math.random 判定失败（0.9 > 0.7）
    vi.spyOn(Math, 'random').mockReturnValue(0.9)

    render(<CaptureScreen onCaptureFail={onFail} />)

    const btn = screen.getByRole('button', { name: /投掷/ })

    // 按住充能到 60%
    fireEvent.pointerDown(btn)
    act(() => {
      vi.advanceTimersByTime(1500) // 30 ticks * 2 = 60%
    })

    // 松手投掷
    fireEvent.pointerUp(btn)

    expect(mockConsumeStamina).toHaveBeenCalledWith(20)

    // 等待投掷动画
    act(() => {
      vi.advanceTimersByTime(600)
    })

    expect(screen.getByText('差一点！')).toBeInTheDocument()
    expect(onFail).toHaveBeenCalledOnce()

    vi.spyOn(Math, 'random').mockRestore()
  })

  it('体力不足时按钮禁用并提示体力不足', () => {
    mockStamina = 10 // 低于 20
    render(<CaptureScreen />)

    expect(screen.getByText('⚡ 体力不足')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /投掷/ })).toBeNull()
  })

  it('消耗体力函数在投掷时被调用', () => {
    vi.spyOn(Math, 'random').mockReturnValue(0.9)

    render(<CaptureScreen />)

    const btn = screen.getByRole('button', { name: /投掷/ })

    // 第一次充能
    fireEvent.pointerDown(btn)
    act(() => {
      vi.advanceTimersByTime(1000)
    })

    expect(mockConsumeStamina).not.toHaveBeenCalled() // 充能时不消耗

    // 松手投掷
    fireEvent.pointerUp(btn)
    expect(mockConsumeStamina).toHaveBeenCalledWith(20)

    vi.spyOn(Math, 'random').mockRestore()
  })

  it('捕获失败后可以重试', () => {
    const onFail = vi.fn()

    vi.spyOn(Math, 'random').mockReturnValue(0.9)

    render(<CaptureScreen onCaptureFail={onFail} />)

    const btn = screen.getByRole('button', { name: /投掷/ })

    // 第一次投掷失败
    fireEvent.pointerDown(btn)
    act(() => {
      vi.advanceTimersByTime(1000)
    })
    fireEvent.pointerUp(btn)
    act(() => {
      vi.advanceTimersByTime(600)
    })

    expect(screen.getByText('差一点！')).toBeInTheDocument()
    expect(onFail).toHaveBeenCalledOnce()

    // 点击"再试一次"
    const retryBtn = screen.getByRole('button', { name: '再试一次' })
    fireEvent.click(retryBtn)

    // 应回到 idle 状态
    expect(screen.getByText('力度 0%')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /投掷/ })).toBeInTheDocument()

    vi.spyOn(Math, 'random').mockRestore()
  })

  it('力度的 power bar 宽度与 charge 百分比一致', () => {
    render(<CaptureScreen />)

    const btn = screen.getByRole('button', { name: /投掷/ })

    // 初始 0%
    expect(screen.getByText('力度 0%')).toBeInTheDocument()

    // 充能到 40%
    fireEvent.pointerDown(btn)
    act(() => {
      vi.advanceTimersByTime(1000) // 20 ticks = 40%
    })
    expect(screen.getByText('力度 40%')).toBeInTheDocument()
  })

  it('充能超过 100% 时不会继续增加', () => {
    render(<CaptureScreen />)

    const btn = screen.getByRole('button', { name: /投掷/ })

    // 按住 3 秒以上（应达到 100% 后停止）
    fireEvent.pointerDown(btn)
    act(() => {
      vi.advanceTimersByTime(3000) // 60 ticks = 120%, but capped at 100%
    })

    expect(screen.getByText('力度 100%')).toBeInTheDocument()
  })

  it('投掷按钮在充能中松手离开按钮也触发投掷', () => {
    vi.spyOn(Math, 'random').mockReturnValue(0.9)
    const onFail = vi.fn()

    render(<CaptureScreen onCaptureFail={onFail} />)

    const btn = screen.getByRole('button', { name: /投掷/ })

    fireEvent.pointerDown(btn)
    act(() => {
      vi.advanceTimersByTime(1000)
    })
    // 鼠标离开按钮区域（pointerLeave 也触发投掷）
    fireEvent.pointerLeave(btn)

    expect(mockConsumeStamina).toHaveBeenCalledWith(20)

    act(() => {
      vi.advanceTimersByTime(600)
    })

    expect(onFail).toHaveBeenCalled()

    vi.spyOn(Math, 'random').mockRestore()
  })
})

// ===== 多物种测试（Issue #35） =====

describe('CaptureScreen — 多物种支持', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    mockStamina = 120
    mockConsumeStamina.mockClear()
    mockGetCaptureBoost.mockClear()
    mockGetCaptureBoost.mockReturnValue(0)
    mockConsumeCaptureBoost.mockClear()
    mockConsumeStamina.mockImplementation((amount: number) => {
      if (mockStamina >= amount) {
        return true
      }
      return false
    })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('cat 物种渲染 🐱 + 🥫', () => {
    render(<CaptureScreen targetSpecies="cat" />)
    expect(screen.getByText('🐱')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /投掷/ })).toHaveTextContent('🥫')
  })

  it('goose 物种渲染 🪿 + 🍞', () => {
    render(<CaptureScreen targetSpecies="goose" />)
    expect(screen.getByText('🪿')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /投掷/ })).toHaveTextContent('🍞')
  })

  it('dog 物种渲染 🐶 + 🦴', () => {
    render(<CaptureScreen targetSpecies="dog" />)
    expect(screen.getByText('🐶')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /投掷/ })).toHaveTextContent('🦴')
  })

  it('捕获成功后 entry.species === targetSpecies', () => {
    const onSuccess = vi.fn()
    // mock 命中成功
    vi.spyOn(Math, 'random').mockReturnValue(0.5)

    render(<CaptureScreen targetSpecies="dog" onCaptureSuccess={onSuccess} />)

    const btn = screen.getByRole('button', { name: /投掷/ })
    fireEvent.pointerDown(btn)
    act(() => {
      vi.advanceTimersByTime(1250) // dog chargeRate 1.5: 25 ticks * 1.5 = 37.5%
    })
    fireEvent.pointerUp(btn)
    act(() => {
      vi.advanceTimersByTime(600)
    })

    expect(onSuccess).toHaveBeenCalledOnce()
    const entryArg = onSuccess.mock.calls[0][0]
    expect(entryArg.species).toBe('dog')

    vi.spyOn(Math, 'random').mockRestore()
  })

  it('不同物种充能速率不同：goose (2.5) 快于 dog (1.5)', () => {
    const onFail = vi.fn()
    vi.spyOn(Math, 'random').mockReturnValue(0.9)

    const { rerender } = render(<CaptureScreen targetSpecies="goose" onCaptureFail={onFail} />)

    const gooseBtn = screen.getByRole('button', { name: /投掷/ })
    fireEvent.pointerDown(gooseBtn)
    // 推进 1000ms = 20 ticks: goose chargeRate 2.5 → 50%
    act(() => {
      vi.advanceTimersByTime(1000)
    })
    const gooseCharge = screen.getByText(/力度 \d+%/).textContent
    fireEvent.pointerUp(gooseBtn)
    act(() => {
      vi.advanceTimersByTime(600)
    })
    // 回到 idle
    const retryBtn = screen.getByRole('button', { name: '再试一次' })
    fireEvent.click(retryBtn)

    // 重新渲染为 dog
    onFail.mockClear()
    rerender(<CaptureScreen targetSpecies="dog" onCaptureFail={onFail} />)

    const dogBtn = screen.getByRole('button', { name: /投掷/ })
    fireEvent.pointerDown(dogBtn)
    // 相同时间：dog chargeRate 1.5 → 30%
    act(() => {
      vi.advanceTimersByTime(1000)
    })
    const dogCharge = screen.getByText(/力度 \d+%/).textContent

    // goose 充能更快
    expect(gooseCharge).toBe('力度 50%')
    expect(dogCharge).toBe('力度 30%')

    vi.spyOn(Math, 'random').mockRestore()
  })
})
