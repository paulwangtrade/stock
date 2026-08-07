/** Paper Trading Observation Dashboard — 只读 HTTP API（Phase6.7-B） */

export type DashboardToday = {
  enabled: boolean
  tradeDate: string
  planCount: number
  planId?: number
  runStatus: string
  trigger?: string
  actor?: string
  executionId?: string
  ordersTotal: number
  filledCount: number
  rejectCount: number
  filledAmount: number
  cash: number
  equity: number
  unrealizedPnl: number
  initialCash: number
  message?: string
  runStartedAt?: string
  runFinishedAt?: string
  dataSourceNote: string
}

export type DashboardPositionRow = {
  stockCode: string
  stockName: string
  totalVolume: number
  availableVolume: number
  lockedVolume: number
  avgCost: number
  markPrice: number
  persistedMarkPrice?: number
  displayPrice?: number
  marketValue: number
  unrealizedPnl: number
  returnRate: number
  updatedAt: string
  quoteSource?: string
  quoteUpdatedAt?: string
  t1Locked: boolean
}

export type DashboardPositions = {
  enabled: boolean
  accountId?: number
  cash: number
  marketValue: number
  equity: number
  unrealizedPnl: number
  initialCash: number
  observationMarketValue?: number
  observationUnrealizedPnl?: number
  observationEquity?: number
  quoteOverlay?: boolean
  positions: DashboardPositionRow[]
  dataSourceNote: string
}

export type DashboardRun = {
  id: number
  executionId: string
  tradeDate: string
  planId: number
  trigger: string
  actor: string
  status: string
  message: string
  ordersTotal: number
  filledCount: number
  rejectCount: number
  skippedAlready: number
  accountId: number
  startedAt: string
  finishedAt?: string
}

export type DashboardRuns = {
  enabled: boolean
  tradeDate?: string
  runs: DashboardRun[]
  total: number
  dataSourceNote: string
}

function num(v: unknown, d = 0): number {
  const n = Number(v)
  return Number.isFinite(n) ? n : d
}

function str(v: unknown, d = ''): string {
  if (v == null) return d
  return String(v)
}

function mapToday(raw: any): DashboardToday {
  return {
    enabled: !!raw?.enabled,
    tradeDate: str(raw?.tradeDate),
    planCount: num(raw?.planCount),
    planId: raw?.planId ? num(raw.planId) : undefined,
    runStatus: str(raw?.runStatus, 'no_run'),
    trigger: raw?.trigger ? str(raw.trigger) : undefined,
    actor: raw?.actor ? str(raw.actor) : undefined,
    executionId: raw?.executionId ? str(raw.executionId) : undefined,
    ordersTotal: num(raw?.ordersTotal),
    filledCount: num(raw?.filledCount),
    rejectCount: num(raw?.rejectCount),
    filledAmount: num(raw?.filledAmount),
    cash: num(raw?.cash),
    equity: num(raw?.equity),
    unrealizedPnl: num(raw?.unrealizedPnl),
    initialCash: num(raw?.initialCash),
    message: raw?.message ? str(raw.message) : undefined,
    runStartedAt: raw?.runStartedAt ? str(raw.runStartedAt) : undefined,
    runFinishedAt: raw?.runFinishedAt ? str(raw.runFinishedAt) : undefined,
    dataSourceNote: str(raw?.dataSourceNote),
  }
}

function mapPosition(raw: any): DashboardPositionRow {
  return {
    stockCode: str(raw?.stockCode),
    stockName: str(raw?.stockName),
    totalVolume: num(raw?.totalVolume),
    availableVolume: num(raw?.availableVolume),
    lockedVolume: num(raw?.lockedVolume),
    avgCost: num(raw?.avgCost),
    markPrice: num(raw?.markPrice),
    persistedMarkPrice: raw?.persistedMarkPrice != null ? num(raw.persistedMarkPrice) : undefined,
    displayPrice: raw?.displayPrice != null ? num(raw.displayPrice) : undefined,
    marketValue: num(raw?.marketValue),
    unrealizedPnl: num(raw?.unrealizedPnl),
    returnRate: num(raw?.returnRate),
    updatedAt: str(raw?.updatedAt),
    quoteSource: raw?.quoteSource ? str(raw.quoteSource) : undefined,
    quoteUpdatedAt: raw?.quoteUpdatedAt ? str(raw.quoteUpdatedAt) : undefined,
    t1Locked: !!raw?.t1Locked,
  }
}

function mapRun(raw: any): DashboardRun {
  return {
    id: num(raw?.id),
    executionId: str(raw?.executionId),
    tradeDate: str(raw?.tradeDate),
    planId: num(raw?.planId),
    trigger: str(raw?.trigger),
    actor: str(raw?.actor),
    status: str(raw?.status),
    message: str(raw?.message),
    ordersTotal: num(raw?.ordersTotal),
    filledCount: num(raw?.filledCount),
    rejectCount: num(raw?.rejectCount),
    skippedAlready: num(raw?.skippedAlready),
    accountId: num(raw?.accountId),
    startedAt: str(raw?.startedAt),
    finishedAt: raw?.finishedAt ? str(raw.finishedAt) : undefined,
  }
}

/** GET /api/papertrading/dashboard/today */
export async function getPaperDashboardToday(tradeDate?: string): Promise<DashboardToday> {
  const q = tradeDate?.trim() ? `?trade_date=${encodeURIComponent(tradeDate.trim())}` : ''
  const res = await fetch(`/api/papertrading/dashboard/today${q}`)
  if (!res.ok) throw new Error(`模拟盘观察今日请求失败: HTTP ${res.status}`)
  const body = await res.json()
  if (!body?.ok) throw new Error(body?.message || '模拟盘观察今日响应无效')
  return mapToday(body.today)
}

/** GET /api/papertrading/dashboard/positions */
export async function getPaperDashboardPositions(): Promise<DashboardPositions> {
  const res = await fetch('/api/papertrading/dashboard/positions')
  if (!res.ok) throw new Error(`模拟盘观察持仓请求失败: HTTP ${res.status}`)
  const body = await res.json()
  if (!body?.ok) throw new Error(body?.message || '模拟盘观察持仓响应无效')
  const raw = body.positions || {}
  return {
    enabled: !!raw.enabled,
    accountId: raw.accountId ? num(raw.accountId) : undefined,
    cash: num(raw.cash),
    marketValue: num(raw.marketValue),
    equity: num(raw.equity),
    unrealizedPnl: num(raw.unrealizedPnl),
    initialCash: num(raw.initialCash),
    observationMarketValue: raw.observationMarketValue != null ? num(raw.observationMarketValue) : undefined,
    observationUnrealizedPnl: raw.observationUnrealizedPnl != null ? num(raw.observationUnrealizedPnl) : undefined,
    observationEquity: raw.observationEquity != null ? num(raw.observationEquity) : undefined,
    quoteOverlay: !!raw.quoteOverlay,
    positions: Array.isArray(raw.positions) ? raw.positions.map(mapPosition) : [],
    dataSourceNote: str(raw.dataSourceNote),
  }
}

/** GET /api/papertrading/dashboard/runs */
export async function getPaperDashboardRuns(opts?: {
  tradeDate?: string
  limit?: number
}): Promise<DashboardRuns> {
  const params = new URLSearchParams()
  if (opts?.tradeDate?.trim()) params.set('trade_date', opts.tradeDate.trim())
  if (opts?.limit) params.set('limit', String(opts.limit))
  const q = params.toString() ? `?${params.toString()}` : ''
  const res = await fetch(`/api/papertrading/dashboard/runs${q}`)
  if (!res.ok) throw new Error(`模拟盘观察运行台账请求失败: HTTP ${res.status}`)
  const body = await res.json()
  if (!body?.ok) throw new Error(body?.message || '模拟盘观察运行台账响应无效')
  const raw = body.runs || {}
  return {
    enabled: !!raw.enabled,
    tradeDate: raw.tradeDate ? str(raw.tradeDate) : undefined,
    runs: Array.isArray(raw.runs) ? raw.runs.map(mapRun) : [],
    total: num(raw.total),
    dataSourceNote: str(raw.dataSourceNote),
  }
}

export type DailyReportRow = {
  id: number
  accountId: number
  reportDate: string
  cash: number
  marketValue: number
  equity: number
  floatingPnl: number
  planCount: number
  orderCount: number
  filledCount: number
  rejectedCount: number
  turnover: number
  maxSinglePositionPct: number
  maxGrossExposurePct: number
  maxSingleStockCode: string
  positionCount: number
  runStatus: string
  createdAt: string
}

export type DailyReportsResponse = {
  enabled: boolean
  reports: DailyReportRow[]
  total: number
  from?: string
  to?: string
}

function mapDailyReport(raw: any): DailyReportRow {
  return {
    id: num(raw?.id),
    accountId: num(raw?.accountId),
    reportDate: str(raw?.reportDate),
    cash: num(raw?.cash),
    marketValue: num(raw?.marketValue),
    equity: num(raw?.equity),
    floatingPnl: num(raw?.floatingPnl),
    planCount: num(raw?.planCount),
    orderCount: num(raw?.orderCount),
    filledCount: num(raw?.filledCount),
    rejectedCount: num(raw?.rejectedCount),
    turnover: num(raw?.turnover),
    maxSinglePositionPct: num(raw?.maxSinglePositionPct),
    maxGrossExposurePct: num(raw?.maxGrossExposurePct),
    maxSingleStockCode: str(raw?.maxSingleStockCode),
    positionCount: num(raw?.positionCount),
    runStatus: str(raw?.runStatus),
    createdAt: str(raw?.createdAt),
  }
}

/** GET /api/papertrading/reports/daily?from=&to= */
export async function getPaperDailyReports(opts?: {
  from?: string
  to?: string
  limit?: number
}): Promise<DailyReportsResponse> {
  const params = new URLSearchParams()
  if (opts?.from?.trim()) params.set('from', opts.from.trim())
  if (opts?.to?.trim()) params.set('to', opts.to.trim())
  if (opts?.limit) params.set('limit', String(opts.limit))
  const q = params.toString() ? `?${params.toString()}` : ''
  const res = await fetch(`/api/papertrading/reports/daily${q}`)
  if (!res.ok) throw new Error(`历史日报请求失败: HTTP ${res.status}`)
  const body = await res.json()
  if (!body?.ok) throw new Error(body?.message || '历史日报响应无效')
  return {
    enabled: !!body.enabled,
    reports: Array.isArray(body.reports) ? body.reports.map(mapDailyReport) : [],
    total: num(body.total),
    from: body.from ? str(body.from) : undefined,
    to: body.to ? str(body.to) : undefined,
  }
}

/** Phase10-C.3-O.2 Observation Metrics (read-only; legacy baseline already isolated server-side). */
export type ObservationSessionDistribution = {
  sessionA: number
  sessionB: number
  sessionC: number
  sessionClosed: number
}

export type ObservationFillPolicy = {
  totalFills: number
  marketOpenFills: number
  marketCloseFills: number
  bWindowTotal: number
  bWindowCloseFills: number
  bWindowOpenFills: number
  bWindowCompliant: number
  bWindowViolation: number
}

export type ObservationQuality = {
  okCount: number
  anomalyCount: number
  incompleteCount: number
  legacyCount: number
}

export type ObservationLegacy = {
  legacyBaselineCount: number
  excludedFillCount: number
}

export type ObservationMetrics = {
  enabled: boolean
  tradeDate?: string
  totalRuns: number
  sessionDistribution: ObservationSessionDistribution
  fillPolicy: ObservationFillPolicy
  quality: ObservationQuality
  legacy: ObservationLegacy
  pricePolicyCompliance: number
}

function mapObservationMetrics(raw: any): ObservationMetrics {
  const sess = raw?.sessionDistribution || {}
  const fill = raw?.fillPolicy || {}
  const quality = raw?.quality || {}
  const legacy = raw?.legacy || {}
  return {
    enabled: !!raw?.enabled,
    tradeDate: raw?.tradeDate ? str(raw.tradeDate) : undefined,
    totalRuns: num(raw?.totalRuns),
    sessionDistribution: {
      sessionA: num(sess.sessionA),
      sessionB: num(sess.sessionB),
      sessionC: num(sess.sessionC),
      sessionClosed: num(sess.sessionClosed),
    },
    fillPolicy: {
      totalFills: num(fill.totalFills),
      marketOpenFills: num(fill.marketOpenFills),
      marketCloseFills: num(fill.marketCloseFills),
      bWindowTotal: num(fill.bWindowTotal),
      bWindowCloseFills: num(fill.bWindowCloseFills),
      bWindowOpenFills: num(fill.bWindowOpenFills),
      bWindowCompliant: num(fill.bWindowCompliant),
      bWindowViolation: num(fill.bWindowViolation),
    },
    quality: {
      okCount: num(quality.okCount),
      anomalyCount: num(quality.anomalyCount),
      incompleteCount: num(quality.incompleteCount),
      legacyCount: num(quality.legacyCount),
    },
    legacy: {
      legacyBaselineCount: num(legacy.legacyBaselineCount),
      excludedFillCount: num(legacy.excludedFillCount),
    },
    pricePolicyCompliance: num(raw?.pricePolicyCompliance, 1),
  }
}

/** GET /api/papertrading/observation/metrics?trade_date= */
export async function getPaperObservationMetrics(tradeDate?: string): Promise<ObservationMetrics> {
  const q = tradeDate?.trim() ? `?trade_date=${encodeURIComponent(tradeDate.trim())}` : ''
  const res = await fetch(`/api/papertrading/observation/metrics${q}`)
  if (!res.ok) throw new Error(`执行观察 metrics 请求失败: HTTP ${res.status}`)
  const body = await res.json()
  if (!body?.ok) throw new Error(body?.message || '执行观察 metrics 响应无效')
  // Prefer nested metrics object; fall back to flat aliases from O.2 handler.
  const raw = body.metrics || {
    enabled: body.enabled,
    tradeDate: body.tradeDate,
    totalRuns: body.totalRuns,
    sessionDistribution: body.sessionDistribution,
    fillPolicy: body.fillPolicy,
    quality: body.quality,
    legacy: body.legacy,
    pricePolicyCompliance: body.pricePolicyCompliance,
  }
  return mapObservationMetrics(raw)
}
