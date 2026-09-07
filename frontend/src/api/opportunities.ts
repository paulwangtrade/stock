/**
 * Phase14-G1.1 — Opportunity user action API client.
 */

export const OPPORTUNITY_ACTION = {
  VIEW: 'VIEW',
  WATCH: 'WATCH',
  IGNORE: 'IGNORE',
} as const

export type OpportunityActionType = (typeof OPPORTUNITY_ACTION)[keyof typeof OPPORTUNITY_ACTION]

export type OpportunityUserActionSummary = {
  id: string
  action: string
  created_at: string
  created_by?: string
  opportunity_id: string
}

export type OpportunityPoolEntry = {
  opportunity_id: string
  scan_batch_key: string
  stock_code: string
  secucode: string
  stock_name?: string
  signal_tag?: string
  signal_time?: string
  latest_user_action?: OpportunityUserActionSummary | null
}

export type SaveOpportunityActionRequest = {
  accountId?: number
  scanBatchKey: string
  opportunityId?: string
  stockCode?: string
  secucode?: string
  signalTime?: string
  signalTag?: string
  action: OpportunityActionType
  createdBy?: string
}

export type OpportunityListQuery = {
  tradeDate?: string
  session?: string
  strategyId?: string
  accountId?: number
  includeUserAction?: boolean
}

function mapActionSummary(raw: Record<string, unknown> | null | undefined) {
  if (!raw?.action) return null
  return {
    id: String(raw.id || ''),
    action: String(raw.action || ''),
    created_at: String(raw.created_at || raw.createdAt || ''),
    created_by: raw.created_by ? String(raw.created_by) : undefined,
    opportunity_id: String(raw.opportunity_id || raw.opportunityId || ''),
  } satisfies OpportunityUserActionSummary
}

function mapEntry(raw: Record<string, unknown>): OpportunityPoolEntry {
  return {
    opportunity_id: String(raw.opportunity_id || raw.opportunityId || ''),
    scan_batch_key: String(raw.scan_batch_key || raw.scanBatchKey || ''),
    stock_code: String(raw.stock_code || raw.stockCode || ''),
    secucode: String(raw.secucode || raw.SECUCODE || ''),
    stock_name: raw.stock_name ? String(raw.stock_name) : undefined,
    signal_tag: raw.signal_tag ? String(raw.signal_tag) : undefined,
    signal_time: raw.signal_time ? String(raw.signal_time) : undefined,
    latest_user_action: mapActionSummary(
      (raw.latest_user_action || raw.latestUserAction) as Record<string, unknown>,
    ),
  }
}

/** POST /api/opportunities/action */
export async function postOpportunityAction(req: SaveOpportunityActionRequest) {
  const res = await fetch('/api/opportunities/action', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      account_id: req.accountId,
      scan_batch_key: req.scanBatchKey,
      opportunity_id: req.opportunityId,
      stock_code: req.stockCode,
      secucode: req.secucode,
      signal_time: req.signalTime,
      signal_tag: req.signalTag,
      action: req.action,
      created_by: req.createdBy || 'ui:opportunity-list',
    }),
  })
  const body = (await res.json()) as Record<string, unknown>
  if (!res.ok || !body?.ok) {
    throw new Error(String(body?.message || `HTTP ${res.status}`))
  }
  return body.action as Record<string, unknown>
}

/** GET /api/opportunities/list */
export async function fetchOpportunityList(q: OpportunityListQuery = {}) {
  const params = new URLSearchParams()
  if (q.tradeDate?.trim()) params.set('trade_date', q.tradeDate.trim())
  if (q.session?.trim()) params.set('session', q.session.trim())
  if (q.strategyId?.trim()) params.set('strategy_id', q.strategyId.trim())
  if (q.accountId && q.accountId > 0) params.set('account_id', String(q.accountId))
  if (q.includeUserAction) params.set('include_user_action', 'true')

  const res = await fetch(`/api/opportunities/list?${params.toString()}`)
  const body = (await res.json()) as Record<string, unknown>
  if (!res.ok || !body?.ok) {
    throw new Error(String(body?.message || `HTTP ${res.status}`))
  }
  const pool = (body.pool || {}) as Record<string, unknown>
  const entriesRaw = Array.isArray(pool.entries) ? pool.entries : []
  return {
    accountId: Number(pool.account_id || pool.accountId) || 0,
    snapshotId: Number(pool.snapshot_id || pool.snapshotId) || 0,
    tradeDate: String(pool.trade_date || pool.tradeDate || ''),
    session: String(pool.session || ''),
    strategyId: String(pool.strategy_id || pool.strategyId || ''),
    scanBatchKey: String(pool.scan_batch_key || pool.scanBatchKey || ''),
    entries: entriesRaw.map((e) => mapEntry(e as Record<string, unknown>)),
    dataSourceNote: pool.data_source_note ? String(pool.data_source_note) : '',
  }
}
