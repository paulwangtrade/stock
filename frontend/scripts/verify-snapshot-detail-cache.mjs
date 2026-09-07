import assert from 'node:assert/strict'
import {
  SNAPSHOT_PAYLOAD_CACHE_MAX,
  getCachedSnapshotPayload,
  setCachedSnapshotPayload,
  invalidateSnapshotPayloadCache,
  loadSnapshotPayloadCached,
  clearSnapshotPayloadCache,
  _snapshotPayloadCacheSize,
  _snapshotPayloadCacheKeys,
} from '../src/utils/snapshotDetailCache.js'

clearSnapshotPayloadCache()

let fetchCount = 0
const fetcher = async (id) => {
  fetchCount += 1
  return { id, items: [{ SECUCODE: '1' }], hitTotal: 1 }
}

const a = await loadSnapshotPayloadCached(7, fetcher)
assert.equal(a.hitTotal, 1)
assert.equal(fetchCount, 1)

const b = await loadSnapshotPayloadCached(7, fetcher)
assert.equal(b, a)
assert.equal(fetchCount, 1, 'second load must use cache')

// 并发同 id 只请求一次
clearSnapshotPayloadCache()
fetchCount = 0
let resolveFetch
const slowFetcher = (id) =>
  new Promise((resolve) => {
    fetchCount += 1
    resolveFetch = () => resolve({ id, hitTotal: id })
  })
const p1 = loadSnapshotPayloadCached(3, slowFetcher)
const p2 = loadSnapshotPayloadCached(3, slowFetcher)
resolveFetch()
const [r1, r2] = await Promise.all([p1, p2])
assert.equal(fetchCount, 1, 'inflight dedupe')
assert.equal(r1.hitTotal, 3)
assert.equal(r2.hitTotal, 3)

// LRU：超过 MAX 淘汰最久未用
clearSnapshotPayloadCache()
for (let i = 1; i <= SNAPSHOT_PAYLOAD_CACHE_MAX; i++) {
  setCachedSnapshotPayload(i, { id: i })
}
assert.equal(_snapshotPayloadCacheSize(), SNAPSHOT_PAYLOAD_CACHE_MAX)
getCachedSnapshotPayload(1) // 刷新 1 为最近使用
setCachedSnapshotPayload(SNAPSHOT_PAYLOAD_CACHE_MAX + 1, { id: SNAPSHOT_PAYLOAD_CACHE_MAX + 1 })
assert.equal(_snapshotPayloadCacheSize(), SNAPSHOT_PAYLOAD_CACHE_MAX)
assert.equal(getCachedSnapshotPayload(2), null, 'oldest unused (2) should be evicted')
assert.ok(getCachedSnapshotPayload(1), 'recently touched (1) should remain')
assert.ok(getCachedSnapshotPayload(SNAPSHOT_PAYLOAD_CACHE_MAX + 1))

const keys = _snapshotPayloadCacheKeys()
assert.equal(keys[keys.length - 1], SNAPSHOT_PAYLOAD_CACHE_MAX + 1)

invalidateSnapshotPayloadCache(1)
assert.equal(getCachedSnapshotPayload(1), null)

clearSnapshotPayloadCache()
assert.equal(_snapshotPayloadCacheSize(), 0)

console.log('snapshotDetailCache LRU: ok')
