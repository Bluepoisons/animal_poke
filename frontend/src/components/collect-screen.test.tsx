import React from 'react'
import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import CollectScreen from './CollectScreen'
import type { CardEntry } from '../types'

// Mock useAnimalStore
const mockAnimals: CardEntry[] = [
  { id: 'c001', no: '#000059', rarity: 'common', species: 'cat', unlocked: true, captureDate: '2026-07-08', location: '海曙区·月湖', lat: 29.87, lng: 121.55, seed: 1, isNew: true },
  { id: 'c002', no: '#000058', rarity: 'uncommon', species: 'goose', unlocked: true, captureDate: '2026-07-07', location: '鄞州区·公园', lat: 29.83, lng: 121.57, seed: 2, isNew: true },
  { id: 'c003', no: '#000057', rarity: 'rare', species: 'dog', unlocked: true, captureDate: '2026-07-06', location: '江北区·滨江', lat: 29.90, lng: 121.53, seed: 3 },
  { id: 'c004', no: '#000056', rarity: 'common', species: 'cat', unlocked: true, captureDate: '2026-07-05', location: '海曙区·老街', lat: 29.86, lng: 121.54, seed: 4 },
  { id: 'c008', no: '#???', rarity: 'common', species: 'goose', unlocked: false, captureDate: '—', location: '待发现', lat: 0, lng: 0, seed: 8 },
  { id: 'c009', no: '#???', rarity: 'common', species: 'dog', unlocked: false, captureDate: '—', location: '待发现', lat: 0, lng: 0, seed: 9 },
]

vi.mock('../hooks/useAnimalStore', () => ({
  useAnimalStore: () => ({
    animals: mockAnimals,
    loading: false,
    addAnimal: vi.fn(),
    markViewed: vi.fn(),
  }),
}))

describe('CollectScreen — 物种筛选', () => {
  const onMapOpen = vi.fn()

  it('物种筛选 tab 渲染（全部/猫/鹅/狗）', () => {
    render(<CollectScreen onMapOpen={onMapOpen} />)

    // 物种 emoji + 标签文本应存在
    expect(screen.getByText('📖 全部')).toBeInTheDocument()
    expect(screen.getByText('🐱 猫')).toBeInTheDocument()
    expect(screen.getByText('🪿 鹅')).toBeInTheDocument()
    expect(screen.getByText('🐶 狗')).toBeInTheDocument()
  })

  it('选择"猫"后仅显示猫物种卡片', () => {
    render(<CollectScreen onMapOpen={onMapOpen} />)

    // 点击猫 tab
    const catTab = screen.getByText('🐱 猫')
    fireEvent.click(catTab)

    // 猫卡片 #000059 应可见
    expect(screen.getByText('#000059')).toBeInTheDocument()
    expect(screen.getByText('#000056')).toBeInTheDocument()

    // 狗卡片 #000057 不应出现
    expect(screen.queryByText('#000057')).toBeNull()
  })

  it('选择"鹅"后仅显示鹅物种卡片', () => {
    render(<CollectScreen onMapOpen={onMapOpen} />)

    const gooseTab = screen.getByText('🪿 鹅')
    fireEvent.click(gooseTab)

    // 鹅卡片编号 #000058 应可见
    expect(screen.getByText('#000058')).toBeInTheDocument()

    // 狗的卡片 #000057 不应出现
    expect(screen.queryByText('#000057')).toBeNull()

    // 猫的卡片 #000059 不应出现
    expect(screen.queryByText('#000059')).toBeNull()
  })

  it('卡片上显示对应物种 emoji', () => {
    render(<CollectScreen onMapOpen={onMapOpen} />)

    // 物种 emoji 应在页面上出现（卡片 + header 计数）
    const catElements = screen.getAllByText('🐱')
    const gooseElements = screen.getAllByText('🪿')
    const dogElements = screen.getAllByText('🐶')

    // 至少应在卡片和计数区域中出现
    expect(catElements.length).toBeGreaterThanOrEqual(2)
    expect(gooseElements.length).toBeGreaterThanOrEqual(1)
    expect(dogElements.length).toBeGreaterThanOrEqual(1)
  })

  it('物种计数统计正确', () => {
    render(<CollectScreen onMapOpen={onMapOpen} />)

    // 猫: 2 unlocked (c001, c004)
    // 鹅: 1 unlocked (c002)
    // 狗: 1 unlocked (c003)
    expect(screen.getByText('🐱 2')).toBeInTheDocument()
    expect(screen.getByText('🪿 1')).toBeInTheDocument()
    expect(screen.getByText('🐶 1')).toBeInTheDocument()
  })
})
