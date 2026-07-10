import { useState, useEffect, useCallback } from 'react'
import { AnimalRepository } from '../db/repositories/animal-repository'
import type { AnimalRecord } from '../db/types'

/** 动物收藏 Hook：仅展示 IndexedDB 真实数据，空库保持空态 */
export function useAnimalStore() {
  const [animals, setAnimals] = useState<AnimalRecord[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        const list = await AnimalRepository.getAll()
        if (!cancelled) setAnimals(list)
      } finally {
        if (!cancelled) setLoading(false)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [])

  const addAnimal = useCallback(async (entry: AnimalRecord) => {
    await AnimalRepository.add(entry)
    setAnimals((prev) => [...prev, entry])
  }, [])

  const markViewed = useCallback(async (id: string) => {
    await AnimalRepository.markViewed(id)
    setAnimals((prev) => prev.map((a) => (a.id === id ? { ...a, isNew: false } : a)))
  }, [])

  return { animals, loading, addAnimal, markViewed }
}
