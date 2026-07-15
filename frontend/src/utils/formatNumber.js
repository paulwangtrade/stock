/** 百分比展示：统一保留 2 位小数 */
export function formatPercent2(value) {
  const n = Number(value)
  if (!Number.isFinite(n)) return '--'
  return n.toFixed(2)
}

/** 带正负号的百分比（涨红跌绿场景用数值本身着色） */
export function formatPercent2Signed(value) {
  const n = Number(value)
  if (!Number.isFinite(n)) return '--'
  const s = n.toFixed(2)
  return n > 0 ? `+${s}` : s
}
