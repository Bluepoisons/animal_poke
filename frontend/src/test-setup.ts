import 'fake-indexeddb/auto'
import '@testing-library/jest-dom'

// 固定 jsdom URL，避免不同环境 origin 漂移
if (typeof window !== 'undefined' && window.location?.href === 'about:blank') {
  // vitest/jsdom default is fine; keep explicit for clarity
}

// 稳定可清理的 localStorage / sessionStorage（jsdom 偶发 clear 非函数）
function createMemoryStorage(): Storage {
  const map = new Map<string, string>()
  return {
    get length() {
      return map.size
    },
    clear() {
      map.clear()
    },
    getItem(key: string) {
      return map.has(key) ? map.get(key)! : null
    },
    key(index: number) {
      return Array.from(map.keys())[index] ?? null
    },
    removeItem(key: string) {
      map.delete(key)
    },
    setItem(key: string, value: string) {
      map.set(String(key), String(value))
    },
  }
}

const local = createMemoryStorage()
const session = createMemoryStorage()
Object.defineProperty(globalThis, 'localStorage', { value: local, configurable: true })
Object.defineProperty(globalThis, 'sessionStorage', { value: session, configurable: true })
if (typeof window !== 'undefined') {
  Object.defineProperty(window, 'localStorage', { value: local, configurable: true })
  Object.defineProperty(window, 'sessionStorage', { value: session, configurable: true })
}

// Canvas mock — 消除 getContext 未实现噪音
HTMLCanvasElement.prototype.getContext = function getContext() {
  return {
    canvas: this,
    fillRect: () => {},
    clearRect: () => {},
    getImageData: () => ({ data: new Uint8ClampedArray(4) }),
    putImageData: () => {},
    createImageData: () => ({ data: new Uint8ClampedArray(4) }),
    setTransform: () => {},
    drawImage: () => {},
    save: () => {},
    fillText: () => {},
    restore: () => {},
    beginPath: () => {},
    moveTo: () => {},
    lineTo: () => {},
    closePath: () => {},
    stroke: () => {},
    translate: () => {},
    scale: () => {},
    rotate: () => {},
    arc: () => {},
    fill: () => {},
    measureText: () => ({ width: 0 }),
    transform: () => {},
    rect: () => {},
    clip: () => {},
  } as unknown as CanvasRenderingContext2D
}

HTMLCanvasElement.prototype.toBlob = function toBlob(cb: BlobCallback) {
  cb(new Blob(['x'], { type: 'image/jpeg' }))
}
HTMLCanvasElement.prototype.toDataURL = () => 'data:image/jpeg;base64,AA=='

// matchMedia
if (!window.matchMedia) {
  window.matchMedia = (query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false,
  })
}

// crypto.randomUUID for older envs
if (typeof crypto !== 'undefined' && !('randomUUID' in crypto)) {
  // @ts-expect-error polyfill
  crypto.randomUUID = () =>
    'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
      const r = (Math.random() * 16) | 0
      const v = c === 'x' ? r : (r & 0x3) | 0x8
      return v.toString(16)
    })
}

afterEach(() => {
  localStorage.clear()
  sessionStorage.clear()
})
