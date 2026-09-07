/**
 * 信号快照详情 LRU 缓存（按 snapshot id）。
 * 历史快照不可变：命中则跳过详情请求与重复解析；最多保留 MAX 条，淘汰最久未用。
 */

export const SNAPSHOT_PAYLOAD_CACHE_MAX = 30

/** @type {Map<number, any>} 插入顺序即 LRU：最旧在前，最新在后 */
const payloadCache = new Map()

/** @type {Map<number, Promise<any>>} 同 id 并发只打一次详情接口 */
const inflight = new Map()

function normalizeId(id) {
  const key = Number(id)
  if (!Number.isFinite(key) || key <= 0) return null
  return key
}

function touch(key, value) {
  if (payloadCache.has(key)) payloadCache.delete(key)
  payloadCache.set(key, value)
  while (payloadCache.size > SNAPSHOT_PAYLOAD_CACHE_MAX) {
    const oldest = payloadCache.keys().next().value
    payloadCache.delete(oldest)
  }
}

export function getCachedSnapshotPayload(id) {
  const key = normalizeId(id)
  if (key == null || !payloadCache.has(key)) return null
  const value = payloadCache.get(key)
  touch(key, value)
  return value
}

export function setCachedSnapshotPayload(id, payload) {
  const key = normalizeId(id)
  if (key == null || !payload) return
  touch(key, payload)
}

export function invalidateSnapshotPayloadCache(id) {
  if (id == null) {
    payloadCache.clear()
    inflight.clear()
    return
  }
  const key = normalizeId(id)
  if (key == null) return
  payloadCache.delete(key)
  inflight.delete(key)
}

/**
 * @param {number|string} id
 * @param {(id: number) => Promise<any>} fetcher 详情接口（后端已解析 ResultJSON → payload）
 */
export async function loadSnapshotPayloadCached(id, fetcher) {
  const key = normalizeId(id)
  if (key == null) return null

  const hit = getCachedSnapshotPayload(key)
  if (hit) return hit

  if (inflight.has(key)) {
    return inflight.get(key)
  }

  const pending = (async () => {
    try {
      const payload = await fetcher(key)
      if (payload) setCachedSnapshotPayload(key, payload)
      return payload || null
    } finally {
      inflight.delete(key)
    }
  })()

  inflight.set(key, pending)
  return pending
}

/** 页面卸载时调用，释放大 payload 占用的内存 */
export function clearSnapshotPayloadCache() {
  invalidateSnapshotPayloadCache()
}

/** @internal 测试用 */
export function _snapshotPayloadCacheSize() {
  return payloadCache.size
}

/** @internal 测试用：返回当前 LRU 从旧到新的 key 列表 */
export function _snapshotPayloadCacheKeys() {
  return [...payloadCache.keys()]
}
