/**
 * 自选虚拟滚动滞回：>=on 开启，<off 关闭，中间保持原状态。
 */
export function nextWatchlistVirtualEnabled(current, count, onThreshold = 80, offThreshold = 50) {
  const n = Number(count) || 0
  if (n >= onThreshold) return true
  if (n < offThreshold) return false
  return !!current
}

/** 将扁平列表按列切成虚拟行 */
export function chunkWatchlistVirtualRows(items, cols) {
  const c = Math.max(1, Number(cols) || 1)
  const list = Array.isArray(items) ? items : []
  const rows = []
  for (let i = 0; i < list.length; i += c) {
    const slice = list.slice(i, i + c)
    rows.push({
      key: slice.map((r, j) => String(r?.['股票代码'] || `${i + j}`)).join('|') + '#' + i,
      startIndex: i,
      cells: slice.map((result, j) => ({ result, cardIndex: i + j })),
    })
  }
  return rows
}
