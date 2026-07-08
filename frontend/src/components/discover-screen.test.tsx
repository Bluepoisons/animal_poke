import React from 'react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import DiscoverScreen from './DiscoverScreen'
import type { DetectionResult } from '../services/visionDetect'

// 用模块级变量控制 mock 行为
let mockDetectResult: DetectionResult | null = null
let mockDetectError: Error | null = null

// pending promise 控制变量
let pendingResolve: ((value: DetectionResult) => void) | null = null
let pendingReject: ((err: Error) => void) | null = null

vi.mock('../services/visionDetect', () => ({
  mockVisionDetector: {
    detect: vi.fn().mockImplementation(() => {
      if (mockDetectError) {
        return Promise.reject(mockDetectError)
      }
      if (mockDetectResult) {
        return Promise.resolve(mockDetectResult)
      }
      // 返回 pending promise 用于测试检测中状态
      return new Promise<DetectionResult>((resolve, reject) => {
        pendingResolve = resolve
        pendingReject = reject
      })
    }),
  },
  getSpeciesThreshold: vi.fn().mockReturnValue(0.85),
}))

describe('DiscoverScreen — VLM 检测集成', () => {
  beforeEach(() => {
    mockDetectResult = null
    mockDetectError = null
    pendingResolve = null
    pendingReject = null

    // Mock getUserMedia
    const mockStream = {
      getTracks: () => [{ stop: vi.fn() }],
    }
    Object.defineProperty(navigator, 'mediaDevices', {
      value: {
        getUserMedia: vi.fn().mockResolvedValue(mockStream),
      },
      writable: true,
      configurable: true,
    })

    // Mock video element methods
    HTMLVideoElement.prototype.play = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(HTMLVideoElement.prototype, 'videoWidth', { value: 640, writable: true })
    Object.defineProperty(HTMLVideoElement.prototype, 'videoHeight', { value: 480, writable: true })

    // Mock canvas
    HTMLCanvasElement.prototype.getContext = vi.fn().mockReturnValue({
      drawImage: vi.fn(),
    } as any)
    HTMLCanvasElement.prototype.toDataURL = vi.fn().mockReturnValue('data:image/jpeg;base64,test')
  })

  it('置信度 ≥ 阈值时显示"开始捕获"', async () => {
    // 设置高分置信度检测结果
    mockDetectResult = { species: 'cat', confidence: 0.92, boundingBox: [0.2, 0.25, 0.4, 0.35] }

    const onConfirm = vi.fn()
    const { container } = render(<DiscoverScreen onConfirm={onConfirm} />)

    // 等待摄像头就绪
    await waitFor(() => {
      expect(screen.getByText('寻找目标…')).toBeInTheDocument()
    }, { timeout: 3000 })

    // 模拟拍照
    const captureBtn = container.querySelector('button.btn-primary')
    expect(captureBtn).not.toBeNull()

    if (captureBtn) {
      await userEvent.click(captureBtn)
    }

    // 等待检测完成
    await waitFor(() => {
      expect(screen.getByText('🐾 开始捕获')).toBeInTheDocument()
    }, { timeout: 3000 })

    expect(screen.getByText(/置信度 92%/)).toBeInTheDocument()
  })

  it('置信度 < 阈值时显示"未发现动物"', async () => {
    mockDetectResult = { species: 'cat', confidence: 0.72, boundingBox: [0.2, 0.25, 0.4, 0.35] }

    const onConfirm = vi.fn()
    const { container } = render(<DiscoverScreen onConfirm={onConfirm} />)

    await waitFor(() => {
      expect(screen.getByText('寻找目标…')).toBeInTheDocument()
    }, { timeout: 3000 })

    const captureBtn = container.querySelector('button.btn-primary')
    if (captureBtn) {
      await userEvent.click(captureBtn)
    }

    await waitFor(() => {
      expect(screen.getByText('未发现动物')).toBeInTheDocument()
    }, { timeout: 3000 })

    expect(screen.queryByText('🐾 开始捕获')).toBeNull()
  })

  it('检测中显示"扫描中…"', async () => {
    // 不设置 mockDetectResult 或 mockDetectError → detect 返回 pending promise
    const { container } = render(<DiscoverScreen onConfirm={vi.fn()} />)

    await waitFor(() => {
      expect(screen.getByText('寻找目标…')).toBeInTheDocument()
    }, { timeout: 3000 })

    const captureBtn = container.querySelector('button.btn-primary')
    if (captureBtn) {
      await userEvent.click(captureBtn)
    }

    // 检测中应显示扫描中…
    await waitFor(() => {
      expect(screen.getByText('🔍 扫描中…')).toBeInTheDocument()
    }, { timeout: 3000 })

    // 清理：resolve pending promise 以避免影响后续测试
    if (pendingResolve) {
      pendingResolve({ species: 'cat', confidence: 0.5, boundingBox: [0, 0, 0, 0] })
    }
  })

  it('检测错误时显示错误提示 + 重试按钮', async () => {
    mockDetectError = new Error('网络错误')

    const { container } = render(<DiscoverScreen onConfirm={vi.fn()} />)

    await waitFor(() => {
      expect(screen.getByText('寻找目标…')).toBeInTheDocument()
    }, { timeout: 3000 })

    const captureBtn = container.querySelector('button.btn-primary')
    if (captureBtn) {
      await userEvent.click(captureBtn)
    }

    await waitFor(() => {
      expect(screen.getByText('检测出错')).toBeInTheDocument()
    }, { timeout: 3000 })

    expect(screen.getByText('🔄 重试')).toBeInTheDocument()
  })

  it('mockVisionDetector.detect 被调用时传入 photoData', async () => {
    mockDetectResult = { species: 'cat', confidence: 0.92, boundingBox: [0.2, 0.25, 0.4, 0.35] }

    const { container } = render(<DiscoverScreen onConfirm={vi.fn()} />)

    await waitFor(() => {
      expect(screen.getByText('寻找目标…')).toBeInTheDocument()
    }, { timeout: 3000 })

    const captureBtn = container.querySelector('button.btn-primary')
    if (captureBtn) {
      await userEvent.click(captureBtn)
    }

    // 等待检测完成
    await waitFor(() => {
      expect(screen.getByText('🐾 开始捕获')).toBeInTheDocument()
    }, { timeout: 3000 })

    // 验证 detect 被调用且参数包含 base64 数据
    const { mockVisionDetector } = await import('../services/visionDetect')
    expect(mockVisionDetector.detect).toHaveBeenCalled()
    const callArg = vi.mocked(mockVisionDetector.detect).mock.calls[0][0]
    expect(callArg).toContain('base64')
  })
})
