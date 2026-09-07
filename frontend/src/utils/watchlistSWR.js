/**
 * 自选股 SWR（Stale-While-Revalidate）辅助：
 * 先用 DB 列表立即出屏，再后台补实时行情。
 */

export function normalizeFollowCode(code) {
  let c = String(code || '')
  if (c.startsWith('us')) {
    c = 'gb_' + c.replace('us', '').toLowerCase()
  }
  return c
}

export function watchlistSortKey(sort, code) {
  return String(sort ?? 0).padStart(8, '0') + '_' + code
}

/**
 * 将 FollowedStock 转为可立即渲染的占位行（可能带旧价/无价）。
 */
export function followedToPlaceholderRow(follow) {
  const code = normalizeFollowCode(follow?.StockCode || follow?.stockCode || '')
  const name = follow?.Name || follow?.name || ''
  const sort = Number(follow?.Sort ?? follow?.sort ?? 0) || 0
  const costPrice = Number(follow?.CostPrice ?? follow?.costPrice ?? 0) || 0
  const volume = Number(follow?.Volume ?? follow?.volume ?? 0) || 0
  const alarmChangePercent = Number(follow?.AlarmChangePercent ?? follow?.alarmChangePercent ?? 0) || 0
  const alarmPrice = Number(follow?.AlarmPrice ?? follow?.alarmPrice ?? 0) || 0
  const followPrice = Number(follow?.FollowPrice ?? follow?.followPrice ?? 0) || 0
  const storedPrice = Number(follow?.Price ?? follow?.price ?? 0)
  const hasStalePrice = Number.isFinite(storedPrice) && storedPrice > 0
  const changePercent = Number(follow?.ChangePercent ?? follow?.changePercent ?? 0)

  return {
    '股票代码': code,
    '股票名称': name,
    '当前价格': hasStalePrice ? String(storedPrice) : '--',
    '昨日收盘价': hasStalePrice ? String(storedPrice) : '',
    '上次当前价格': hasStalePrice ? storedPrice : 0,
    '买一报价': '',
    '卖一报价': '',
    changePercent: Number.isFinite(changePercent) ? changePercent : 0,
    changePrice: Number(follow?.PriceChange ?? follow?.priceChange ?? 0) || 0,
    costPrice,
    costVolume: volume,
    profit: 0,
    profitAmount: 0,
    profitAmountToday: 0,
    '关注价格': followPrice,
    followProfit: null,
    sort,
    alarmChangePercent,
    alarmPrice,
    Groups: follow?.Groups || follow?.groups || [],
    quotePending: true,
    quoteFailed: false,
    type: 'default',
    color: '#FFFFFF',
    key: watchlistSortKey(sort, code),
  }
}

/**
 * @returns {{ codes: string[], rowsByKey: Record<string, object> }}
 */
export function buildWatchlistPlaceholders(follows) {
  const list = Array.isArray(follows) ? follows : []
  const codes = []
  const rowsByKey = {}
  for (const follow of list) {
    const row = followedToPlaceholderRow(follow)
    if (!row['股票代码']) continue
    if (!codes.includes(row['股票代码'])) codes.push(row['股票代码'])
    rowsByKey[row.key] = row
  }
  return { codes, rowsByKey }
}

/** 行情失败时：保留列表，仅把仍 pending 的行标为失败并显示 -- */
export function markPendingQuotesFailed(rowsByKey) {
  const src = rowsByKey && typeof rowsByKey === 'object' ? rowsByKey : {}
  const next = { ...src }
  for (const key of Object.keys(next)) {
    const row = next[key]
    if (!row?.quotePending) continue
    next[key] = {
      ...row,
      quotePending: false,
      quoteFailed: true,
      '当前价格': row['当前价格'] && row['当前价格'] !== '' ? row['当前价格'] : '--',
      type: 'default',
      color: '#FFFFFF',
    }
  }
  return next
}

/** 实时行情行写入后清除 pending/failed */
export function clearQuoteFlags(row) {
  if (!row || typeof row !== 'object') return row
  return {
    ...row,
    quotePending: false,
    quoteFailed: false,
  }
}
