/**
 * 筛选页表格性能：分页上限、虚拟滚动滞回、防抖延迟常量。
 */

export const SCREEN_TABLE_PAGE_SIZES = [20, 30, 50]
export const SCREEN_TABLE_DEFAULT_PAGE_SIZE = 30
export const SCREEN_FILTER_DEBOUNCE_MS = 300

/** 虚拟滚动：总数或当前页行数 >= on 开启，< off 关闭 */
export const SCREEN_VIRTUAL_ON = 100
export const SCREEN_VIRTUAL_OFF = 60

export function nextScreenVirtualEnabled(current, count, onThreshold = SCREEN_VIRTUAL_ON, offThreshold = SCREEN_VIRTUAL_OFF) {
  const n = Number(count) || 0
  if (n >= onThreshold) return true
  if (n < offThreshold) return false
  return !!current
}

export function clampScreenPageSize(pageSize, allowed = SCREEN_TABLE_PAGE_SIZES, fallback = SCREEN_TABLE_DEFAULT_PAGE_SIZE) {
  const n = Number(pageSize)
  if (allowed.includes(n)) return n
  return fallback
}
