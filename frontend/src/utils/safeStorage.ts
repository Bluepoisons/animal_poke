let storageAvailable: boolean | null = null

function isStorageAvailable(): boolean {
  if (storageAvailable !== null) return storageAvailable
  try {
    const testKey = '__storage_test__'
    localStorage.setItem(testKey, '1')
    localStorage.removeItem(testKey)
    storageAvailable = true
  } catch {
    storageAvailable = false
  }
  return storageAvailable
}

export function safeGetItem<T>(key: string, fallback: T): T {
  if (!isStorageAvailable()) return fallback
  try {
    const value = localStorage.getItem(key)
    if (value === null) return fallback
    return JSON.parse(value) as T
  } catch {
    return fallback
  }
}

export function safeSetItem(key: string, value: unknown): boolean {
  if (!isStorageAvailable()) return false
  try {
    localStorage.setItem(key, JSON.stringify(value))
    return true
  } catch (err) {
    if (err instanceof DOMException && err.name === 'QuotaExceededError') {
      try {
        localStorage.removeItem(key)
        localStorage.setItem(key, JSON.stringify(value))
        return true
      } catch {
        return false
      }
    }
    return false
  }
}

export function safeRemoveItem(key: string): void {
  if (!isStorageAvailable()) return
  try {
    localStorage.removeItem(key)
  } catch {
    // 忽略
  }
}
