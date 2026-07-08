import 'fake-indexeddb/auto'
import '@testing-library/jest-dom'

// jsdom 环境下补全 matchMedia（部分组件可能用到）
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
