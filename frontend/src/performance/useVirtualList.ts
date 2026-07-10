import { useCallback, useMemo, useState } from 'react'
import { virtualWindow } from './logic'

export interface UseVirtualListOptions {
  total: number
  itemHeight: number
  viewportHeight: number
  overscan?: number
}

/** Hook for virtualized collection lists (AP-054) */
export function useVirtualList({
  total,
  itemHeight,
  viewportHeight,
  overscan = 4,
}: UseVirtualListOptions) {
  const [scrollTop, setScrollTop] = useState(0)

  const onScroll = useCallback((e: { currentTarget: { scrollTop: number } }) => {
    setScrollTop(e.currentTarget.scrollTop)
  }, [])

  const window = useMemo(
    () => virtualWindow(total, scrollTop, viewportHeight, itemHeight, overscan),
    [total, scrollTop, viewportHeight, itemHeight, overscan],
  )

  const totalHeight = total * itemHeight

  return {
    scrollTop,
    onScroll,
    start: window.start,
    end: window.end,
    offsetY: window.offsetY,
    totalHeight,
  }
}
