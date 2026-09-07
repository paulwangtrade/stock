/** GET /api/portfolio/snapshot — shared read model for observation / 我的组合. Display overlay never feeds equity. */

import { positionStateLabel } from '../utils/positionStateDisplay.js'

function num(v: unknown, d = 0): number {
  const n = Number(v)
  return Number.isFinite(n) ? n : d
}

function str(v: unknown, d = ''): string {
  if (v == null) return d
  return String(v)
}

function nullableNum(v: unknown): number | null {
  if (v === null || v === undefined || v === '') return null
  const n = Number(v)
  return Number.isFinite(n) ? n : null
}

export type SnapshotPositionRow = {
  stockCode: string
  stockName: string
  totalQty: number
  availableQty: number
  lockedQty: number
  avgCost: number
  markPrice: number
  marketValue: number
  pnl: number
  pnlPercent: number | null
  displayPrice: number | null
  displayQuoteSource: string
  displayMarketValue: number | null
  displayPnl: number | null
  displayPnlPercent: number | null
  /** Quote.PreClose when overlay present; null if missing. */
  quotePreClose: number | null
  /**
   * Single-name today PnL: (display_price - quote_pre_close) × total_qty.
   * Null when either price missing — never invent 0. ≠ account daily_pnl / ledger pnl.
   */
  todayPnl: number | null
  /** Live/open quote only; null when overlay falls back to mark (Phase17.2). */
  quotePrice: number | null
  /** Quote FetchedAt ISO; display freshness only. */
  quoteTimestamp: string | null
  /** FRESH | STALE | UNKNOWN — quote overlay, not ledger mark. */
  priceFreshness: string
  positionStatus: string
  positionStatusLabel: string
  isNewPosition: boolean
  canSell?: boolean
}

export type PortfolioSnapshotView = {
  found: boolean
  cash: number
  equity: number
  marketValue: number
  positionCount: number
  /** Ledger Σ pnl (mark). Never sum of display_pnl. */
  ledgerPnl: number
  quoteOverlay: boolean
  tradeDate: string
  /** Snapshot build / query clock (ISO); display freshness only. */
  asOf: string
  updatedAt: string
  dataSourceNote: string
  disclaimer: string
  positions: SnapshotPositionRow[]
}

/** Map one Snapshot position. totalQty is Snapshot.total_qty — never available+locked. */
export function mapSnapshotPosition(raw: any): SnapshotPositionRow | null {
  if (!raw || typeof raw !== 'object') return null
  const stockCode = str(raw.stock_code ?? raw.stockCode).trim()
  if (!stockCode) return null
  const ps = raw.position_state && typeof raw.position_state === 'object' ? raw.position_state : {}
  const state = str(ps.state)
  const totalQty = num(raw.total_qty ?? raw.totalQty)
  const availableQty = num(raw.available_qty ?? raw.availableQty)
  const lockedQty = num(raw.locked_qty ?? raw.lockedQty)
  return {
    stockCode,
    stockName: str(raw.stock_name ?? raw.stockName),
    totalQty,
    availableQty,
    lockedQty,
    avgCost: num(raw.avg_cost ?? raw.avgCost),
    markPrice: num(raw.mark_price ?? raw.markPrice),
    marketValue: num(raw.market_value ?? raw.marketValue),
    pnl: num(raw.pnl),
    pnlPercent: nullableNum(raw.pnl_percent ?? raw.pnlPercent),
    displayPrice: nullableNum(raw.display_price ?? raw.displayPrice),
    displayQuoteSource: str(raw.display_quote_source ?? raw.displayQuoteSource),
    displayMarketValue: nullableNum(raw.display_market_value ?? raw.displayMarketValue),
    displayPnl: nullableNum(raw.display_pnl ?? raw.displayPnl),
    displayPnlPercent: nullableNum(raw.display_pnl_percent ?? raw.displayPnlPercent),
    quotePreClose: nullableNum(raw.quote_pre_close ?? raw.quotePreClose),
    todayPnl: nullableNum(raw.today_pnl ?? raw.todayPnl),
    quotePrice: nullableNum(raw.quote_price ?? raw.quotePrice),
    quoteTimestamp: (() => {
      const t = raw.quote_timestamp ?? raw.quoteTimestamp
      if (t == null || t === '') return null
      return String(t)
    })(),
    priceFreshness: str(raw.price_freshness ?? raw.priceFreshness, 'UNKNOWN'),
    positionStatus: state,
    positionStatusLabel: positionStateLabel(state),
    isNewPosition: !!ps.is_new_position,
    canSell: ps.can_sell != null ? !!ps.can_sell : undefined,
  }
}

export function mapPortfolioSnapshot(raw: any): PortfolioSnapshotView {
  const list = Array.isArray(raw?.positions) ? raw.positions : []
  const positions = list.map(mapSnapshotPosition).filter((p): p is SnapshotPositionRow => !!p)
  const overlay = positions.some(
    (p) => p.displayQuoteSource === 'live' || p.displayQuoteSource === 'open_fallback',
  )
  let ledgerPnl = 0
  for (const p of positions) {
    ledgerPnl += p.pnl
  }
  return {
    found: !!raw?.found,
    cash: num(raw?.cash),
    equity: num(raw?.equity),
    marketValue: num(raw?.market_value ?? raw?.marketValue),
    positionCount: num(raw?.position_count ?? raw?.positionCount, positions.length),
    ledgerPnl,
    quoteOverlay: overlay,
    tradeDate: str(raw?.trade_date ?? raw?.tradeDate),
    asOf: str(raw?.as_of ?? raw?.asOf),
    updatedAt: str(raw?.updated_at ?? raw?.updatedAt),
    dataSourceNote: str(raw?.data_source_note ?? raw?.dataSourceNote),
    disclaimer: str(raw?.disclaimer),
    positions,
  }
}

/** GET /api/portfolio/snapshot — observation uses include_display=1 for row prices only. */
export async function getPortfolioSnapshot(opts?: {
  tradeDate?: string
  includeDisplay?: boolean
}): Promise<PortfolioSnapshotView> {
  const params = new URLSearchParams()
  if (opts?.tradeDate?.trim()) params.set('trade_date', opts.tradeDate.trim())
  if (opts?.includeDisplay) params.set('include_display', '1')
  const q = params.toString() ? `?${params.toString()}` : ''
  const res = await fetch(`/api/portfolio/snapshot${q}`)
  if (!res.ok) throw new Error(`组合快照请求失败: HTTP ${res.status}`)
  const body = await res.json()
  if (!body?.ok) throw new Error(body?.message || '组合快照响应无效')
  return mapPortfolioSnapshot(body.snapshot)
}
