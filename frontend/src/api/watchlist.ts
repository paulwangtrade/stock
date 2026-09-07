/**
 * Phase16.26-C2.3.2-A — GET /api/watchlist (read-only watching list + trade_plan_id).
 */

export type WatchlistItem = {
  opportunity_id?: string
  stock_code: string
  stock_name?: string
  scan_batch_key: string
  source?: string
  watch_time: string
  latest_action: string
  in_trade_plan?: boolean
  trade_plan_status?: string
  trade_plan_id?: number
}

export type WatchlistView = {
  account_id: number
  watching_count: number
  items: WatchlistItem[]
  data_source_note?: string
}

function mapItem(raw: Record<string, unknown>): WatchlistItem {
  const status = String(raw.trade_plan_status || raw.tradePlanStatus || 'NONE').toUpperCase() || 'NONE'
  const planId = Math.trunc(Number(raw.trade_plan_id ?? raw.tradePlanId) || 0)
  return {
    opportunity_id: raw.opportunity_id ? String(raw.opportunity_id) : undefined,
    stock_code: String(raw.stock_code || raw.stockCode || ''),
    stock_name: raw.stock_name ? String(raw.stock_name) : raw.stockName ? String(raw.stockName) : undefined,
    scan_batch_key: String(raw.scan_batch_key || raw.scanBatchKey || ''),
    source: raw.source ? String(raw.source) : undefined,
    watch_time: String(raw.watch_time || raw.watchTime || ''),
    latest_action: String(raw.latest_action || raw.latestAction || ''),
    in_trade_plan: Boolean(raw.in_trade_plan ?? raw.inTradePlan),
    trade_plan_status: status,
    trade_plan_id: planId > 0 ? planId : undefined,
  }
}

/** GET /api/watchlist */
export async function fetchWatchlist(opts: { accountId?: number; limit?: number } = {}): Promise<WatchlistView> {
  const params = new URLSearchParams()
  if (opts.accountId && opts.accountId > 0) params.set('account_id', String(opts.accountId))
  if (opts.limit && opts.limit > 0) params.set('limit', String(opts.limit))
  const qs = params.toString()
  const res = await fetch(`/api/watchlist${qs ? `?${qs}` : ''}`)
  const body = (await res.json()) as Record<string, unknown>
  if (!res.ok || !body?.ok) {
    throw new Error(String(body?.message || `HTTP ${res.status}`))
  }
  const wl = (body.watchlist || {}) as Record<string, unknown>
  const itemsRaw = Array.isArray(wl.items) ? wl.items : []
  return {
    account_id: Number(wl.account_id || wl.accountId) || 0,
    watching_count: Number(wl.watching_count || wl.watchingCount) || 0,
    items: itemsRaw.map((e) => mapItem(e as Record<string, unknown>)),
    data_source_note: wl.data_source_note ? String(wl.data_source_note) : undefined,
  }
}
