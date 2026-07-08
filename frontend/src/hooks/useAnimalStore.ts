import { useState, useEffect, useCallback } from 'react'
import { AnimalRepository } from '../db/repositories/animal-repository'
import { MOCK_ENTRIES } from '../types'
import type { AnimalRecord } from '../db/types'

/** 动物收藏 Hook：从 IndexedDB 加载数据，空库时写入 mock 初始数据 */
export function useAnimalStore() {
  const [animals, setAnimals] = useState<AnimalRecord[]>([])
  const [loading, setLoading] = useState(true)

  // 初始化加载
  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        let list = await AnimalRepository.getAll()
        // 空数据库时写入 mock 数据作为初始展示
        if (list.length === 0) {
          await AnimalRepository.bulkAdd(MOCK_ENTRIES)
          list = await AnimalRepository.getAll()
        }
        if (!cancelled) {
          setAnimals(list)
        }
      } finally {
        if (!cancelled) {
          setLoading(false)
        }
      }
    })()
    return () => { cancelled = true }
  }, [])

  // 新增动物
  const addAnimal = useCallback(async (entry: AnimalRecord) => {
    await AnimalRepository.add(entry)
    setAnimals(prev => [...prev, entry])
  }, [])

  // 标记已查看
  const markViewed = useCallback(async (id: string) => {
    await AnimalRepository.markViewed(id)
    setAnimals(prev => prev.map(a => (a.id === id ? { ...a, isNew: false } : a)))
  }, [])

  return { animals, loading, addAnimal, markViewed }
}
