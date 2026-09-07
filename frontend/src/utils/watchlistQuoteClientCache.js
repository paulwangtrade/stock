/**
 * 前端自选行情短期缓存：5 秒内同分组不重复请求 GetFollowRealtimeList。
 */

const DEFAULT_QUOTE_CLIENT_TTL_MS = 5000

let quoteCacheGroupId = null
let quoteCacheAt = 0
let quoteCachePayload = null
let quoteCacheTTL = DEFAULT_QUOTE_CLIENT_TTL_MS
let quoteInflight = null
let quoteInflightGroupId = null

export function setWatchlistQuoteClientTTL(ms) {
  const n = Number(ms)
  quoteCacheTTL = Number.isFinite(n) && n > 0 ? n : DEFAULT_QUOTE_CLIENT_TTL_MS
}

export function getWatchlistQuoteClientTTL() {
  return quoteCacheTTL
}

export function clearWatchlistQuoteClientCache() {
  quoteCacheGroupId = null
  quoteCacheAt = 0
  quoteCachePayload = null
  quoteInflight = null
  quoteInflightGroupId = null
}

export function peekWatchlistQuoteClientCache(groupId) {
  if (quoteCachePayload == null) return null
  if (Number(quoteCacheGroupId) !== Number(groupId)) return null
  if (Date.now() - quoteCacheAt >= quoteCacheTTL) return null
  return quoteCachePayload
}

/**
 * @param {number} groupId
 * @param {(id: number) => Promise<any>} fetcher
 * @param {{ force?: boolean }} [opts]
 */
export async function getFollowRealtimeCached(groupId, fetcher, opts = {}) {
  const force = !!opts.force
  const gid = Number(groupId) || 0

  if (!force) {
    const hit = peekWatchlistQuoteClientCache(gid)
    if (hit != null) return hit
    if (quoteInflight && quoteInflightGroupId === gid) {
      return quoteInflight
    }
  }

  const req = Promise.resolve()
    .then(() => fetcher(gid))
    .then((data) => {
      quoteCacheGroupId = gid
      quoteCacheAt = Date.now()
      quoteCachePayload = data
      return data
    })
    .finally(() => {
      if (quoteInflight === req) {
        quoteInflight = null
        quoteInflightGroupId = null
      }
    })

  quoteInflight = req
  quoteInflightGroupId = gid
  return req
}
