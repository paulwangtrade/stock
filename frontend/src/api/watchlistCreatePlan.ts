/**
 * POST /api/tradeplans/watchlist-draft — Watchlist → buy TradePlan Draft (Phase16.26-C2.3.2-B1).
 */

export const WATCHLIST_DRAFT_ERROR_CODES = {
  PLAN_EXISTS: 'PLAN_EXISTS',
  NOT_WATCHING: 'NOT_WATCHING',
  INVALID_STOCK: 'INVALID_STOCK',
  MISSING_FIELDS: 'MISSING_FIELDS',
} as const

const ERROR_MESSAGES: Record<string, string> = {
  [WATCHLIST_DRAFT_ERROR_CODES.PLAN_EXISTS]: '该股票已有交易计划，请查看计划（禁止重复创建）',
  [WATCHLIST_DRAFT_ERROR_CODES.NOT_WATCHING]: '该机会已不在跟踪中，无法创建计划',
  [WATCHLIST_DRAFT_ERROR_CODES.INVALID_STOCK]: '股票代码无效',
  [WATCHLIST_DRAFT_ERROR_CODES.MISSING_FIELDS]: '缺少机会标识或扫描批次',
  INVALID_JSON: '请求格式无效',
  BAD_REQUEST: '请求无效',
}

export type WatchlistDraftRequest = {
  stock_code: string
  opportunity_id: string
  scan_batch_key: string
  stock_name?: string
  trade_date?: string
  actor?: string
  account_id?: number
}

export type WatchlistDraftResponse = {
  ok: boolean
  plan_id: number
  status: string
  side: string
  trade_date: string
  plan_version: number
  source_session: string
  item_count?: number
  message?: string
}

export class WatchlistDraftError extends Error {
  code: string
  httpStatus: number
  userMessage: string
  planId?: number
  tradePlanStatus?: string

  constructor(opts: {
    code: string
    message: string
    httpStatus: number
    planId?: number
    tradePlanStatus?: string
  }) {
    super(opts.message)
    this.name = 'WatchlistDraftError'
    this.code = opts.code
    this.httpStatus = opts.httpStatus
    this.planId = opts.planId
    this.tradePlanStatus = opts.tradePlanStatus
    this.userMessage = ERROR_MESSAGES[opts.code] || opts.message || '创建交易计划失败'
  }
}

/** POST /api/tradeplans/watchlist-draft */
export async function createWatchlistTradePlanDraft(
  payload: WatchlistDraftRequest,
): Promise<WatchlistDraftResponse> {
  const body = {
    stock_code: String(payload.stock_code || '').trim(),
    opportunity_id: String(payload.opportunity_id || '').trim(),
    scan_batch_key: String(payload.scan_batch_key || '').trim(),
    ...(payload.stock_name ? { stock_name: String(payload.stock_name).trim() } : {}),
    ...(payload.trade_date ? { trade_date: String(payload.trade_date).trim() } : {}),
    actor: String(payload.actor || 'ui:watched-opportunities').trim(),
    ...(payload.account_id && payload.account_id > 0 ? { account_id: payload.account_id } : {}),
  }
  const res = await fetch('/api/tradeplans/watchlist-draft', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  const raw = (await res.json().catch(() => ({}))) as Record<string, unknown>
  if (!res.ok || !raw?.ok) {
    throw new WatchlistDraftError({
      code: String(raw?.code || 'BAD_REQUEST'),
      message: String(raw?.message || `HTTP ${res.status}`),
      httpStatus: res.status,
      planId: Math.trunc(Number(raw?.plan_id) || 0) || undefined,
      tradePlanStatus: raw?.trade_plan_status ? String(raw.trade_plan_status) : undefined,
    })
  }
  return {
    ok: true,
    plan_id: Math.trunc(Number(raw.plan_id) || 0),
    status: String(raw.status || 'draft'),
    side: String(raw.side || 'buy'),
    trade_date: String(raw.trade_date || ''),
    plan_version: Math.trunc(Number(raw.plan_version) || 0),
    source_session: String(raw.source_session || 'watchlist'),
    item_count: Math.trunc(Number(raw.item_count) || 0),
    message: raw.message ? String(raw.message) : undefined,
  }
}
