import assert from 'node:assert/strict'
import {
  clearFollowListClientCache,
  getFollowListCached,
  invalidateFollowListCache,
  isFollowListCacheFresh,
  peekFollowListCache,
  setFollowListClientTTL,
} from '../src/utils/watchlistFollowListCache.js'

clearFollowListClientCache()
setFollowListClientTTL(80)

let calls = 0
const fetcher = async (id) => {
  calls++
  return [{ groupId: id, n: calls }]
}

const a = await getFollowListCached(0, fetcher)
const b = await getFollowListCached(0, fetcher)
assert.equal(calls, 1)
assert.deepEqual(a, b)
assert.equal(isFollowListCacheFresh(0), true)

// 分组隔离
const c = await getFollowListCached(2, fetcher)
assert.equal(calls, 2)
assert.equal(c[0].groupId, 2)

await new Promise((r) => setTimeout(r, 100))
assert.equal(isFollowListCacheFresh(0), false)
const stale = peekFollowListCache(0, { allowStale: true })
assert.ok(stale)
assert.equal(peekFollowListCache(0), null)

const d = await getFollowListCached(0, fetcher)
assert.equal(calls, 3)
assert.equal(d[0].n, 3)

const e = await getFollowListCached(0, fetcher, { force: true })
assert.equal(calls, 4)
assert.equal(e[0].n, 4)

invalidateFollowListCache(0)
assert.equal(peekFollowListCache(0, { allowStale: true }), null)
assert.ok(peekFollowListCache(2, { allowStale: true }))

invalidateFollowListCache()
assert.equal(peekFollowListCache(2, { allowStale: true }), null)

// 并发去重
clearFollowListClientCache()
setFollowListClientTTL(5000)
calls = 0
const slow = async (id) => {
  calls++
  await new Promise((r) => setTimeout(r, 30))
  return [{ id }]
}
const [x, y] = await Promise.all([
  getFollowListCached(1, slow),
  getFollowListCached(1, slow),
])
assert.equal(calls, 1)
assert.deepEqual(x, y)

console.log('verify-watchlist-follow-list-cache: ok')
