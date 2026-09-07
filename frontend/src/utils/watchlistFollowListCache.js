/**
 * 前端自选股列表短期缓存（按分组）：
 * - TTL 默认 30s，命中则不打 GetFollowList
 * - 过期仍可 peek 旧数据（切换分组先闪旧再后台更新）
 * - 添加/删除自选后 invalidate
 */

import { clearWatchlistQuoteClientCache } from './watchlistQuoteClientCache.js'

const DEFAULT_FOLLOW_LIST_TTL_MS = 30000

/** @type {Map<number, { data: any[], timestamp: number }>} */
const followListCache = new Map()

let followListTTL = DEFAULT_FOLLOW_LIST_TTL_MS
/** @type {Map<number, Promise<any[]>>} */
const followListInflight = new Map()

export function setFollowListClientTTL(ms) {
  const n = Number(ms)
  followListTTL = Number.isFinite(n) && n > 0 ? n : DEFAULT_FOLLOW_LIST_TTL_MS
}

export function getFollowListClientTTL() {
  return followListTTL
}

export function invalidateFollowListCache(groupId) {
  if (groupId === undefined || groupId === null) {
    followListCache.clear()
    followListInflight.clear()
  } else {
    const gid = Number(groupId) || 0
    followListCache.delete(gid)
    followListInflight.delete(gid)
  }
  // 成员变化后行情缓存也失效，避免展示已移除标的
  clearWatchlistQuoteClientCache()
}

export function clearFollowListClientCache() {
  invalidateFollowListCache()
}

/**
 * @param {number} groupId
 * @param {{ allowStale?: boolean }} [opts]
 * @returns {any[]|null}
 */
export function peekFollowListCache(groupId, opts = {}) {
  const gid = Number(groupId) || 0
  const entry = followListCache.get(gid)
  if (!entry) return null
  const fresh = Date.now() - entry.timestamp < followListTTL
  if (!fresh && !opts.allowStale) return null
  return entry.data
}

export function isFollowListCacheFresh(groupId) {
  const gid = Number(groupId) || 0
  const entry = followListCache.get(gid)
  if (!entry) return false
  return Date.now() - entry.timestamp < followListTTL
}

function putFollowListCache(groupId, data) {
  const gid = Number(groupId) || 0
  followListCache.set(gid, {
    data: Array.isArray(data) ? data : [],
    timestamp: Date.now(),
  })
}

/**
 * @param {number} groupId
 * @param {(id: number) => Promise<any>} fetcher
 * @param {{ force?: boolean }} [opts]
 */
export async function getFollowListCached(groupId, fetcher, opts = {}) {
  const force = !!opts.force
  const gid = Number(groupId) || 0

  if (!force) {
    const fresh = peekFollowListCache(gid, { allowStale: false })
    if (fresh != null) return fresh
    const inflight = followListInflight.get(gid)
    if (inflight) return inflight
  }

  const req = Promise.resolve()
    .then(() => fetcher(gid))
    .then((data) => {
      const list = Array.isArray(data) ? data : []
      putFollowListCache(gid, list)
      return list
    })
    .finally(() => {
      if (followListInflight.get(gid) === req) {
        followListInflight.delete(gid)
      }
    })

  followListInflight.set(gid, req)
  return req
}
