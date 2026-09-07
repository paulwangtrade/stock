/** GET /api/portfolio/positions/{stock_code}/provenance — Phase15-A2 read-only holding provenance. */

export type ProvenanceStatus = 'complete' | 'partial'

export type ProvenanceTradeRow = {
  planId: number
  planItemId?: number
  fillId?: number
  fillPrice: number
  fillVolume?: number
  filledAt?: string
  tradeDate?: string
}

export type ProvenanceOriginRow = {
  planId: number
  strategy: string
  signal: {
    tag?: string
    time?: string
    price?: string
    snapshotId?: number
  }
  reason: {
    source?: string
    selection?: string
    item?: string
  }
  score?: string
}

export type PortfolioProvenanceView = {
  stockCode: string
  stockName?: string
  position: {
    quantity: number
    avgCost: number
    markPrice?: number
    unrealizedPnl?: number
  }
  trades: ProvenanceTradeRow[]
  origins: ProvenanceOriginRow[]
  reconcile?: {
    status: string
    positionVolume: number
    attributedVolume: number
  }
  provenanceStatus: ProvenanceStatus
  asOf?: string
  dataSourceNote?: string
  disclaimer?: string
}

export class PortfolioProvenanceNotFoundError extends Error {
  readonly status = 404

  constructor(message = 'position not found') {
    super(message)
    this.name = 'PortfolioProvenanceNotFoundError'
  }
}

function num(v: unknown, d = 0): number {
  const n = Number(v)
  return Number.isFinite(n) ? n : d
}

function str(v: unknown, d = ''): string {
  if (v == null) return d
  return String(v)
}

function mapTrade(raw: Record<string, unknown>): ProvenanceTradeRow {
  return {
    planId: num(raw.plan_id ?? raw.planId),
    planItemId: num(raw.plan_item_id ?? raw.planItemId) || undefined,
    fillId: num(raw.fill_id ?? raw.fillId) || undefined,
    fillPrice: num(raw.fill_price ?? raw.fillPrice),
    fillVolume: num(raw.fill_volume ?? raw.fillVolume) || undefined,
    filledAt: str(raw.filled_at ?? raw.filledAt) || undefined,
    tradeDate: str(raw.trade_date ?? raw.tradeDate) || undefined,
  }
}

function mapOrigin(raw: Record<string, unknown>): ProvenanceOriginRow {
  const signal =
    raw.signal && typeof raw.signal === 'object' ? (raw.signal as Record<string, unknown>) : {}
  const reason =
    raw.reason && typeof raw.reason === 'object' ? (raw.reason as Record<string, unknown>) : {}
  const snapshotId = num(signal.snapshot_id ?? signal.snapshotId)
  return {
    planId: num(raw.plan_id ?? raw.planId),
    strategy: str(raw.strategy),
    signal: {
      tag: str(signal.tag) || undefined,
      time: str(signal.time) || undefined,
      price: str(signal.price) || undefined,
      snapshotId: snapshotId > 0 ? snapshotId : undefined,
    },
    reason: {
      source: str(reason.source) || undefined,
      selection: str(reason.selection) || undefined,
      item: str(reason.item) || undefined,
    },
    score: str(raw.score) || undefined,
  }
}

function mapReconcile(raw: Record<string, unknown> | undefined) {
  if (!raw || typeof raw !== 'object') return undefined
  return {
    status: str(raw.status),
    positionVolume: num(raw.position_volume ?? raw.positionVolume),
    attributedVolume: num(raw.attributed_volume ?? raw.attributedVolume),
  }
}

/** Compute complete vs partial from trades + origins (frontend-only status). */
export function computeProvenanceStatus(view: {
  trades?: ProvenanceTradeRow[]
  origins?: ProvenanceOriginRow[]
  reconcile?: { status?: string }
}): ProvenanceStatus {
  const trades = Array.isArray(view.trades) ? view.trades : []
  const origins = Array.isArray(view.origins) ? view.origins : []
  if (trades.length === 0) return 'partial'
  if (view.reconcile?.status && view.reconcile.status !== 'matched') return 'partial'

  for (const trade of trades) {
    if (!trade.planId) return 'partial'
    const origin = origins.find((o) => o.planId === trade.planId)
    if (!origin) return 'partial'
    const strategy = String(origin.strategy || '').trim()
    if (!strategy) return 'partial'
    const hasSignal =
      !!String(origin.signal?.tag || '').trim() ||
      !!String(origin.signal?.time || '').trim() ||
      (origin.signal?.snapshotId ?? 0) > 0
    const hasReason = !!String(origin.reason?.source || '').trim()
    if (!hasSignal && !hasReason) return 'partial'
  }
  return 'complete'
}

export function mapPortfolioProvenance(raw: Record<string, unknown>): PortfolioProvenanceView {
  const trades = Array.isArray(raw.trades) ? raw.trades.map((t) => mapTrade(t as Record<string, unknown>)) : []
  const origins = Array.isArray(raw.origins)
    ? raw.origins.map((o) => mapOrigin(o as Record<string, unknown>))
    : []
  const reconcile = mapReconcile(raw.reconcile as Record<string, unknown> | undefined)
  const base = {
    stockCode: str(raw.stock_code ?? raw.stockCode),
    stockName: str(raw.stock_name ?? raw.stockName) || undefined,
    position: {
      quantity: num((raw.position as Record<string, unknown>)?.quantity),
      avgCost: num((raw.position as Record<string, unknown>)?.avg_cost ?? (raw.position as Record<string, unknown>)?.avgCost),
      markPrice: num((raw.position as Record<string, unknown>)?.mark_price ?? (raw.position as Record<string, unknown>)?.markPrice) || undefined,
      unrealizedPnl:
        num((raw.position as Record<string, unknown>)?.unrealized_pnl ?? (raw.position as Record<string, unknown>)?.unrealizedPnl) ||
        undefined,
    },
    trades,
    origins,
    reconcile,
    asOf: str(raw.as_of ?? raw.asOf) || undefined,
    dataSourceNote: str(raw.data_source_note ?? raw.dataSourceNote) || undefined,
    disclaimer: str(raw.disclaimer) || undefined,
  }
  return {
    ...base,
    provenanceStatus: computeProvenanceStatus(base),
  }
}

/** GET /api/portfolio/positions/{stock_code}/provenance */
export async function getPortfolioProvenance(stockCode: string): Promise<PortfolioProvenanceView> {
  const code = String(stockCode || '').trim()
  if (!code) {
    throw new PortfolioProvenanceNotFoundError('stock code required')
  }
  const res = await fetch(`/api/portfolio/positions/${encodeURIComponent(code)}/provenance`)
  const body = await res.json().catch(() => ({}))
  if (res.status === 404 || (body && body.ok === false && Number(body.code) === 404)) {
    throw new PortfolioProvenanceNotFoundError(body?.message || 'position not found')
  }
  if (!res.ok) {
    throw new Error(body?.message || `持仓来源请求失败: HTTP ${res.status}`)
  }
  if (!body?.ok) {
    throw new Error(body?.message || '持仓来源响应无效')
  }
  const raw = body.provenance
  if (!raw || typeof raw !== 'object') {
    throw new Error('持仓来源响应无效')
  }
  return mapPortfolioProvenance(raw as Record<string, unknown>)
}
