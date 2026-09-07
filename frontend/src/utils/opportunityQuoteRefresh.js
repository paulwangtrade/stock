/** Phase14-H2.1: opportunity list live quote refresh helpers (pure, testable). */

export function collectQuoteCodesFromRows(rows, resolveCode) {
  const resolver = typeof resolveCode === 'function' ? resolveCode : (row) => row?.SECUCODE || ''
  const seen = new Set()
  const codes = []
  for (const row of rows || []) {
    if (!row?.SECUCODE) continue
    const code = String(resolver(row) || '').trim()
    if (!code || seen.has(code)) continue
    seen.add(code)
    codes.push(code)
  }
  return codes
}

export function isValidCachedQuote(entry) {
  if (!entry || typeof entry !== 'object') return false
  const price = Number(entry.price)
  return Number.isFinite(price) && price > 0
}

/**
 * Decide which codes need a network fetch.
 * Skips codes with valid cache entries or already in-flight.
 */
export function planQuoteRefresh(codes, quoteCache, inFlightCodes) {
  const cache = quoteCache instanceof Map ? quoteCache : new Map(Object.entries(quoteCache || {}))
  const inFlight = inFlightCodes instanceof Set ? inFlightCodes : new Set(inFlightCodes || [])

  const toFetch = []
  const skippedCached = []
  const skippedInFlight = []

  for (const code of codes || []) {
    const key = String(code || '').trim()
    if (!key) continue
    if (isValidCachedQuote(cache.get(key))) {
      skippedCached.push(key)
      continue
    }
    if (inFlight.has(key)) {
      skippedInFlight.push(key)
      continue
    }
    toFetch.push(key)
  }

  return { toFetch, skippedCached, skippedInFlight }
}

export function parseLiveQuoteResponse(quote) {
  const price = Number(quote?.price)
  if (quote?.code !== 0 || !Number.isFinite(price) || price <= 0) {
    return null
  }
  const changeRate = Number(quote?.changePercent ?? quote?.change_percent)
  return {
    price,
    changeRate: Number.isFinite(changeRate) ? changeRate : null,
  }
}

export function mergeQuoteIntoCache(quoteCache, code, entry) {
  const key = String(code || '').trim()
  const base = quoteCache instanceof Map ? quoteCache : new Map(Object.entries(quoteCache || {}))
  const next = new Map(base)
  if (key && entry) {
    next.set(key, entry)
  }
  return next
}

export function shouldRefreshSnapshotQuotes({ signalDataSource, filteredRowCount }) {
  return signalDataSource === 'snapshot' && Number(filteredRowCount) > 0
}

/** Aggregate plan for diagnostics / tests. */
export function resolveQuoteRefreshState(codes, quoteCache, inFlightCodes) {
  const plan = planQuoteRefresh(codes, quoteCache, inFlightCodes)
  return {
    ...plan,
    pendingCount: plan.toFetch.length,
    skipCount: plan.skippedCached.length + plan.skippedInFlight.length,
  }
}
