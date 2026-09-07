/**
 * Opportunity user action UI helpers (Phase14-G1.1).
 */

export const OPPORTUNITY_ACTION_WATCH = 'WATCH'

/** Scan batch key aligned with backend opportunity.BatchKeyFromSnapshot. */
export function batchKeyFromSnapshot(snap) {
  if (!snap) return ''
  const id = Number(snap.id) || 0
  if (id > 0) return `snap:${id}`
  return `${snap.tradeDate || snap.trade_date || ''}|${snap.session || ''}|${snap.strategyId || snap.strategy_id || ''}`
}

/** Build lookup map secucode -> pool entry from GET list response. */
export function indexOpportunityEntriesBySecucode(entries) {
  const map = Object.create(null)
  for (const e of entries || []) {
    const key = String(e?.secucode || e?.SECUCODE || '').trim().toUpperCase()
    if (key) map[key] = e
  }
  return map
}

export function resolveOpportunityEntryForRow(row, entryBySecucode) {
  const secu = String(row?.SECUCODE || '').trim().toUpperCase()
  return secu ? entryBySecucode?.[secu] || null : null
}

export function isOpportunityWatched(entry) {
  const action = String(
    entry?.latest_user_action?.action || entry?.latestUserAction?.action || '',
  )
    .trim()
    .toUpperCase()
  return action === OPPORTUNITY_ACTION_WATCH
}

export function buildWatchActionPayload({ snap, row, signalSummary }) {
  const batchKey = batchKeyFromSnapshot(snap)
  const secucode = String(row?.SECUCODE || '').trim()
  const signalTime = String(
    row?.SIGNAL_TIME ?? row?.signal_time ?? signalSummary?.signalTime ?? '',
  ).trim()
  const signalTag = String(signalSummary?.tag ?? row?.tag ?? '').trim()
  return {
    scanBatchKey: batchKey,
    secucode,
    signalTime,
    signalTag,
  }
}

/** Same batch/secucode fields as WATCH; used for append-only IGNORE (cancel track). */
export function buildIgnoreActionPayload(args) {
  return buildWatchActionPayload(args)
}

/**
 * Phase16.26-C2.2 — IGNORE payload from watchlist list item (no snap / SECUCODE).
 * Returns null if required fields are missing.
 */
export function buildIgnoreFromWatchlistItem(item) {
  const scanBatchKey = String(item?.scan_batch_key || item?.scanBatchKey || '').trim()
  const opportunityId = String(item?.opportunity_id || item?.opportunityId || '').trim()
  const stockCode = String(item?.stock_code || item?.stockCode || '').trim()
  if (!scanBatchKey || !opportunityId || !stockCode) return null
  return {
    scanBatchKey,
    opportunityId,
    stockCode,
  }
}
