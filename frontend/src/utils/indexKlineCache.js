import { GetStockEastMoneyKLine } from '../../wailsjs/go/main/App'

/**
 * 共享指数日 K 请求单例：MarketStatusBar / watchlistSignalScan 等共用。
 * - TTL 内命中内存缓存
 * - 同 key in-flight Promise 去重，避免并发重复 HTTP
 */
const DEFAULT_TTL_MS = 60_000
const cache = new Map()
const inflight = new Map()

function cacheKey(indexCode, klt, barCount) {
  return `${String(indexCode || '').toUpperCase()}|${klt}|${barCount}`
}

/**
 * @param {string} indexCode 如 000001.SH
 * @param {string} indexName
 * @param {object} [opts]
 * @param {string} [opts.klt='101']
 * @param {number} [opts.barCount=120]
 * @param {number} [opts.ttlMs]
 * @returns {Promise<Array>}
 */
export function getSharedIndexDailyKLine(indexCode, indexName = '', opts = {}) {
  const klt = opts.klt || '101'
  const barCount = Number(opts.barCount) > 0 ? Number(opts.barCount) : 120
  const ttlMs = Number(opts.ttlMs) > 0 ? Number(opts.ttlMs) : DEFAULT_TTL_MS
  const code = String(indexCode || '').trim()
  if (!code) return Promise.resolve([])

  const key = cacheKey(code, klt, barCount)
  const hit = cache.get(key)
  if (hit && Date.now() - hit.at < ttlMs) {
    return Promise.resolve(hit.bars)
  }

  const pending = inflight.get(key)
  if (pending) return pending

  const promise = (async () => {
    try {
      const raw = await GetStockEastMoneyKLine(code, indexName || '', klt, barCount)
      const bars = Array.isArray(raw) ? raw : []
      cache.set(key, { bars, at: Date.now() })
      return bars
    } catch {
      if (hit?.bars) return hit.bars
      return []
    } finally {
      inflight.delete(key)
    }
  })()

  inflight.set(key, promise)
  return promise
}

/** 测试/调试用 */
export function clearIndexKlineCache() {
  cache.clear()
  inflight.clear()
}

export function getIndexKlineCacheStats() {
  return { size: cache.size, inflight: inflight.size }
}
