/** Portfolio Dashboard v0 — read-only (Phase11-B.3). Calls GET /api/portfolio/dashboard only. */

export type PortfolioSummaryView = {
  equity: number
  cash: number
  marketValue: number
  positionCount: number
  dailyPnl: number | null
  dailyPnlBasis: string
}

export type PortfolioPositionRow = {
  stockCode: string
  stockName: string
  quantity: number
  avgCost: number
  marketPrice: number
  unrealizedPnl: number
}

export type PortfolioRiskView = {
  maxPositionRatio: number
  industryConcentration: number | null
  riskLevel: string
}

export type PortfolioTradeFillRow = {
  fillId: number
  orderId: number
  planId: number
  stockCode: string
  stockName: string
  side: string
  price: number
  volume: number
  fee: number
  fillReason: string
  filledAt: string
}

export type PortfolioDashboardView = {
  tradeDate: string
  /** Dashboard assemble clock (ISO); display freshness only. */
  asOf: string
  found: boolean
  summary: PortfolioSummaryView
  positions: PortfolioPositionRow[]
  risk: PortfolioRiskView
  trades: {
    tradeDate: string
    fills: PortfolioTradeFillRow[]
  }
  dataSourceNote: string
  disclaimer: string
}

function num(v: unknown, d = 0): number {
  const n = Number(v)
  return Number.isFinite(n) ? n : d
}

function nullableNum(v: unknown): number | null {
  if (v === null || v === undefined || v === '') return null
  const n = Number(v)
  return Number.isFinite(n) ? n : null
}

function str(v: unknown, d = ''): string {
  if (v == null) return d
  return String(v)
}

function mapDashboard(raw: any): PortfolioDashboardView {
  const s = raw?.summary || {}
  const risk = raw?.risk || {}
  const trades = raw?.trades || {}
  const fills = Array.isArray(trades.fills) ? trades.fills : []
  const positions = Array.isArray(raw?.positions) ? raw.positions : []
  return {
    tradeDate: str(raw?.trade_date),
    asOf: str(raw?.as_of ?? raw?.asOf),
    found: !!raw?.found,
    summary: {
      equity: num(s.equity),
      cash: num(s.cash),
      marketValue: num(s.market_value),
      positionCount: num(s.position_count),
      dailyPnl: nullableNum(s.daily_pnl),
      dailyPnlBasis: str(s.daily_pnl_basis),
    },
    positions: positions.map((p: any) => ({
      stockCode: str(p.stock_code),
      stockName: str(p.stock_name),
      quantity: num(p.quantity),
      avgCost: num(p.avg_cost),
      marketPrice: num(p.market_price),
      unrealizedPnl: num(p.unrealized_pnl),
    })),
    risk: {
      maxPositionRatio: num(risk.max_position_ratio),
      industryConcentration: nullableNum(risk.industry_concentration),
      riskLevel: str(risk.risk_level, 'UNKNOWN'),
    },
    trades: {
      tradeDate: str(trades.trade_date),
      fills: fills.map((f: any) => ({
        fillId: num(f.fill_id),
        orderId: num(f.order_id),
        planId: num(f.plan_id),
        stockCode: str(f.stock_code),
        stockName: str(f.stock_name),
        side: str(f.side),
        price: num(f.price),
        volume: num(f.volume),
        fee: num(f.fee),
        fillReason: str(f.fill_reason),
        filledAt: str(f.filled_at),
      })),
    },
    dataSourceNote: str(raw?.data_source_note),
    disclaimer: str(raw?.disclaimer),
  }
}

/** GET /api/portfolio/dashboard */
export async function getPortfolioDashboard(tradeDate?: string): Promise<PortfolioDashboardView> {
  const q = tradeDate ? `?trade_date=${encodeURIComponent(tradeDate)}` : ''
  const res = await fetch(`/api/portfolio/dashboard${q}`)
  if (!res.ok) {
    throw new Error(`portfolio dashboard HTTP ${res.status}`)
  }
  const body = await res.json()
  if (!body?.ok) {
    throw new Error(body?.message || 'portfolio dashboard failed')
  }
  return mapDashboard(body.dashboard)
}
