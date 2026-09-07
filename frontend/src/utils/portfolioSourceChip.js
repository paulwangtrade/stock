/**
 * Phase16.27-P2 — Portfolio holdings source chip (UI only).
 * Maps existing TradePlan.source_session → Strategy | Watchlist | Manual.
 * Does not add provenance DTO fields or change execution.
 */

export const PROVENANCE_SOURCE_BUCKET = {
  strategy: 'strategy',
  watchlist: 'watchlist',
  manual: 'manual',
  unknown: 'unknown',
}

/** Compact chip copy required by P2 acceptance cases. */
export const PROVENANCE_SOURCE_CHIP = {
  strategy: { label: 'Strategy', type: 'info' },
  watchlist: { label: 'Watchlist', type: 'warning' },
  manual: { label: 'Manual', type: 'default' },
  unknown: { label: '未知来源', type: 'default' },
}

/**
 * @param {unknown} sourceSession trade_plans.source_session (existing field)
 * @returns {'strategy'|'watchlist'|'manual'|'unknown'}
 */
export function resolveProvenanceSourceBucket(sourceSession) {
  const s = String(sourceSession || '').trim().toLowerCase()
  if (!s) return PROVENANCE_SOURCE_BUCKET.unknown
  if (s === 'watchlist') return PROVENANCE_SOURCE_BUCKET.watchlist
  if (s === 't_sell' || s === 'exit_review') return PROVENANCE_SOURCE_BUCKET.manual
  if (s === 'after_close' || s === 'morning_rebuild' || s === 'cash_rescale') {
    return PROVENANCE_SOURCE_BUCKET.strategy
  }
  return PROVENANCE_SOURCE_BUCKET.unknown
}

export function provenanceSourceChipMeta(bucket) {
  const key = PROVENANCE_SOURCE_CHIP[bucket] ? bucket : PROVENANCE_SOURCE_BUCKET.unknown
  return PROVENANCE_SOURCE_CHIP[key]
}

/** Prefer newest trade (filledAt desc) with planId > 0. */
export function pickLatestPlanIdFromTrades(trades) {
  const list = Array.isArray(trades) ? [...trades] : []
  list.sort((a, b) => String(b?.filledAt || b?.filled_at || '').localeCompare(String(a?.filledAt || a?.filled_at || '')))
  for (const t of list) {
    const id = Math.trunc(Number(t?.planId ?? t?.plan_id) || 0)
    if (id > 0) return id
  }
  return 0
}
