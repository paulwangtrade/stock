/**
 * 筛选页历史快照：默认选中「最新可用」辅助逻辑（纯函数，便于单测）。
 */

export function parseSnapshotTime(value) {
  if (!value) return null
  const d = value instanceof Date ? value : new Date(value)
  return Number.isNaN(d.getTime()) ? null : d
}

/** 按 createdAt DESC，优先 status=done 且 hitTotal>0，否则最近 done */
export function pickBestUsableSnapshot(list) {
  const sorted = [...(list || [])].sort((a, b) => {
    const ta = parseSnapshotTime(a?.createdAt)?.getTime() ?? 0
    const tb = parseSnapshotTime(b?.createdAt)?.getTime() ?? 0
    return tb - ta
  })
  const withHits = sorted.find((s) => String(s?.status || '') === 'done' && Number(s?.hitTotal) > 0)
  if (withHits) return withHits
  return sorted.find((s) => String(s?.status || '') === 'done') || null
}

/** 相对今天的整天数；无效时间返回 null */
export function snapshotAgeDays(createdAt, now = Date.now()) {
  const created = parseSnapshotTime(createdAt)
  if (!created) return null
  return Math.floor((now - created.getTime()) / 86400000)
}

export function buildSnapshotStaleBannerText(createdAt, staleDays = 7, now = Date.now()) {
  const days = snapshotAgeDays(createdAt, now)
  if (days == null || days < staleDays) return ''
  return `当前数据更新于 ${days} 天前，建议重新扫描`
}
