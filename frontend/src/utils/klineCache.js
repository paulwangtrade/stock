/** Short-lived in-memory K-line cache (frontend only; does not change API). */

import { getChinaTimeParts, isKlineMarketLive } from './aShareSessionClock.js'

const DEFAULT_TTL_MS = 5 * 60 * 1000
/** Phase17-B.1 IDLE：同进程反复打开同周期，拉长内存命中。 */
const IDLE_TTL_MS = 30 * 60 * 1000

const WEEKDAY_NUM = { Mon: 1, Tue: 2, Wed: 3, Thu: 4, Fri: 5, Sat: 6, Sun: 0 }

/** @type {Map<string, { expiresAt: number, data: unknown }>} */
const store = new Map()

/** @type {Map<string, Promise<unknown>>} */
const pendingRequests = new Map()

function isDevEnv() {
  try {
    return Boolean(import.meta?.env?.DEV)
  } catch {
    return false
  }
}

function logCache(event, key) {
  if (!isDevEnv()) return
  // eslint-disable-next-line no-console
  console.debug(`[KLINE_CACHE] ${event}`, key)
}

/** LIVE 5min / IDLE 30min — does not affect backend TTL. */
export function resolveKlineCacheTtlMs(date = new Date()) {
  return isKlineMarketLive(date) ? DEFAULT_TTL_MS : IDLE_TTL_MS
}

/**
 * @param {string} symbol e.g. sh600363
 * @param {string} timeframe e.g. 5
 * @param {string} [date] YYYY-MM-DD; defaults to Asia/Shanghai today
 */
export function klineCacheKey(symbol, timeframe, date) {
  const sym = String(symbol || '').trim().toLowerCase()
  const tf = String(timeframe || '').trim()
  const d = date || shanghaiDateString()
  return `${sym}|${tf}|${d}`
}

function normalizeBarDayKey(day) {
  return String(day || '')
    .replace(/[-/: T]/g, '')
    .slice(0, 8)
}

/** Phase17.1: mirror backend calendar gate lightly (weekday only; no holiday table). */
export function feKlinePayloadCalendarFresh(data, date = new Date()) {
  if (!Array.isArray(data) || data.length === 0) return false
  const lastRaw = data[data.length - 1]?.Day ?? data[data.length - 1]?.day ?? ''
  const last = normalizeBarDayKey(lastRaw)
  if (!last) return false
  const { weekday, hour, minute } = getChinaTimeParts(date)
  const wd = WEEKDAY_NUM[weekday]
  if (!wd || wd >= 6) return true
  const hm = Number(hour) * 60 + Number(minute)
  if (hm < 9 * 60 + 30) return true
  const today = normalizeBarDayKey(shanghaiDateString(date))
  return last >= today
}

export function getCachedKline(key, ttlMs = DEFAULT_TTL_MS) {
  const entry = store.get(String(key || ''))
  if (!entry) return null
  if (Date.now() > entry.expiresAt) {
    store.delete(String(key))
    return null
  }
  // ttlMs kept for API compat (expiry already stored at set time)
  void ttlMs
  return entry.data
}

export function setCachedKline(key, data, ttlMs = DEFAULT_TTL_MS) {
  const k = String(key || '')
  if (!k) return
  store.set(k, { data, expiresAt: Date.now() + Math.max(1000, ttlMs) })
}

export function clearKlineCache() {
  store.clear()
  pendingRequests.clear()
}

/**
 * Cache-aside + in-flight dedupe. Does not cache thrown errors.
 *
 * @template T
 * @param {string} key
 * @param {() => Promise<T>} fetcher
 * @param {number} [ttlMs]
 * @param {{ shouldCache?: (data: T) => boolean }} [opts]
 * @returns {Promise<T>}
 */
export async function getOrFetch(key, fetcher, ttlMs = DEFAULT_TTL_MS, opts = {}) {
  const k = String(key || '')
  if (!k) {
    return fetcher()
  }
  const shouldCache =
    typeof opts.shouldCache === 'function' ? opts.shouldCache : () => true
  const effectiveTtl =
    ttlMs == null || ttlMs === DEFAULT_TTL_MS ? resolveKlineCacheTtlMs() : ttlMs

  const cached = getCachedKline(k, effectiveTtl)
  if (cached != null) {
    if (feKlinePayloadCalendarFresh(cached)) {
      logCache('hit', k)
      return /** @type {T} */ (cached)
    }
    store.delete(k)
    logCache('stale-calendar', k)
  }

  const pending = pendingRequests.get(k)
  if (pending) {
    logCache('pending-hit', k)
    return /** @type {Promise<T>} */ (pending)
  }

  logCache('miss', k)
  const promise = (async () => {
    try {
      const data = await fetcher()
      if (shouldCache(data)) {
        setCachedKline(k, data, effectiveTtl)
      }
      return data
    } finally {
      pendingRequests.delete(k)
    }
  })()

  pendingRequests.set(k, promise)
  return /** @type {Promise<T>} */ (promise)
}

/** @returns {{ size: number, pending: number }} */
export function getKlineCacheStats() {
  return { size: store.size, pending: pendingRequests.size }
}

function shanghaiDateString(date = new Date()) {
  return new Intl.DateTimeFormat('en-CA', {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).format(date)
}

export { DEFAULT_TTL_MS as KLINE_CACHE_DEFAULT_TTL_MS, IDLE_TTL_MS as KLINE_CACHE_IDLE_TTL_MS }
