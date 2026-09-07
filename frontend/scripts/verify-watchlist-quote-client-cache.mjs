import assert from 'node:assert/strict'
import {
  clearWatchlistQuoteClientCache,
  getFollowRealtimeCached,
  peekWatchlistQuoteClientCache,
  setWatchlistQuoteClientTTL,
} from '../src/utils/watchlistQuoteClientCache.js'

clearWatchlistQuoteClientCache()
setWatchlistQuoteClientTTL(50)

let calls = 0
const fetcher = async (id) => {
  calls++
  return [{ groupId: id, price: calls }]
}

const a = await getFollowRealtimeCached(0, fetcher)
const b = await getFollowRealtimeCached(0, fetcher)
assert.equal(calls, 1)
assert.deepEqual(a, b)
assert.ok(peekWatchlistQuoteClientCache(0))

await new Promise((r) => setTimeout(r, 60))
const c = await getFollowRealtimeCached(0, fetcher)
assert.equal(calls, 2)
assert.equal(c[0].price, 2)

const d = await getFollowRealtimeCached(0, fetcher, { force: true })
assert.equal(calls, 3)
assert.equal(d[0].price, 3)

// 并发去重
clearWatchlistQuoteClientCache()
setWatchlistQuoteClientTTL(5000)
calls = 0
const slow = async () => {
  calls++
  await new Promise((r) => setTimeout(r, 30))
  return [{ ok: true }]
}
const [x, y] = await Promise.all([
  getFollowRealtimeCached(1, slow),
  getFollowRealtimeCached(1, slow),
])
assert.equal(calls, 1)
assert.deepEqual(x, y)

console.log('verify-watchlist-quote-client-cache: ok')
