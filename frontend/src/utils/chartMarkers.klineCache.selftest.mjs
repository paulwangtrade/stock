import assert from 'node:assert/strict'
import {
  buildIntradayChartModel,
  chartMarkerClass,
  chartMarkerLabel,
} from './chartMarkers.js'
import {
  getCachedKline,
  getOrFetch,
  klineCacheKey,
  setCachedKline,
  clearKlineCache,
  getKlineCacheStats,
} from './klineCache.js'

assert.equal(chartMarkerLabel('BUY'), 'T买')
assert.equal(chartMarkerLabel('SELL'), 'T卖')
assert.equal(chartMarkerClass('BUY'), 'marker-buy')

const bars = [
  { day: '2026-08-12 09:35', close: 10, low: 9.8, high: 10.2 },
  { day: '2026-08-12 09:40', close: 10.5, low: 10, high: 10.6 },
]
const model = buildIntradayChartModel(bars, [
  { time: '09:40', price: 10.5, type: 'BUY', reason: 'future' },
])
assert.ok(model.points.includes(','))
assert.equal(model.markers.length, 1)
assert.equal(model.markers[0].label, 'T买')

clearKlineCache()
const key = klineCacheKey('sh600363', '5', '2026-08-12')
assert.equal(getCachedKline(key), null)
setCachedKline(key, [{ close: 1 }], 60_000)
assert.deepEqual(getCachedKline(key), [{ close: 1 }])

clearKlineCache()
let fetchCount = 0
const fetchKey = klineCacheKey('300123.sz', '101', '2026-09-04')
const slowFetch = () =>
  new Promise((resolve) => {
    fetchCount += 1
    setTimeout(() => resolve([{ close: 42 }]), 30)
  })

const [a, b] = await Promise.all([
  getOrFetch(fetchKey, slowFetch, 60_000),
  getOrFetch(fetchKey, slowFetch, 60_000),
])
assert.deepEqual(a, [{ close: 42 }])
assert.deepEqual(b, [{ close: 42 }])
assert.equal(fetchCount, 1, 'in-flight should dedupe')

const c = await getOrFetch(fetchKey, slowFetch, 60_000)
assert.deepEqual(c, [{ close: 42 }])
assert.equal(fetchCount, 1, 'cache hit should not refetch')

clearKlineCache()
let failCount = 0
await assert.rejects(
  () =>
    getOrFetch(klineCacheKey('x', '101', '2026-09-04'), async () => {
      failCount += 1
      throw new Error('boom')
    }),
  /boom/,
)
assert.equal(getKlineCacheStats().pending, 0)
assert.equal(getCachedKline(klineCacheKey('x', '101', '2026-09-04')), null)

clearKlineCache()
const emptyKey = klineCacheKey('empty', '101', '2026-09-04')
const empty = await getOrFetch(emptyKey, async () => [], 60_000, {
  shouldCache: (data) => Array.isArray(data) && data.length > 0,
})
assert.deepEqual(empty, [])
assert.equal(getCachedKline(emptyKey), null)

clearKlineCache()
const { resolveKlineCacheTtlMs, KLINE_CACHE_DEFAULT_TTL_MS, KLINE_CACHE_IDLE_TTL_MS } =
  await import('./klineCache.js')
const { isKlineMarketLive } = await import('./aShareSessionClock.js')
const liveMon = new Date('2026-09-07T02:00:00.000Z') // Mon 10:00 CST
const idleSun = new Date('2026-09-06T14:00:00.000Z') // Sun 22:00 CST
assert.equal(isKlineMarketLive(liveMon), true)
assert.equal(isKlineMarketLive(idleSun), false)
assert.equal(resolveKlineCacheTtlMs(liveMon), KLINE_CACHE_DEFAULT_TTL_MS)
assert.equal(resolveKlineCacheTtlMs(idleSun), KLINE_CACHE_IDLE_TTL_MS)

console.log('chartMarkers.klineCache.selftest: OK')
