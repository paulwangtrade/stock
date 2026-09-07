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

/** Position Attribution (Phase10) — read-only lot projection */

export type AttributionLot = {
  planId: number
  planItemId: number
  orderId: number
  fillId: number
  fillPrice: number
  volume: number
  costAmount: number
  tradeDate?: string
  strategyName?: string
}

export type AttributionReconcile = {
  positionVolume: number
  attributedVolume: number
  unattributedVolume: number
  surplusFillVolume?: number
  status: string
}

export type AttributionUnattributed = {
  volume: number
  reasonCode: string
  message: string
}

export type AttributionPositionRow = {
  stockCode: string
  stockName: string
  totalVolume: number
  currentPrice: number
  pnl: number
  lots: AttributionLot[]
  reconcile: AttributionReconcile
  unattributed?: AttributionUnattributed
  sourceSummary?: string
  planIds?: number[]
}

export type PositionAttributionView = {
  enabled: boolean
  accountId?: number
  asOf?: string
  reconcileAllMatched: boolean
  positions: AttributionPositionRow[]
  dataSourceNote: string
}

function mapAttributionLot(raw: any): AttributionLot {
  return {
    planId: num(raw?.plan_id ?? raw?.planId),
    planItemId: num(raw?.plan_item_id ?? raw?.planItemId),
    orderId: num(raw?.order_id ?? raw?.orderId),
    fillId: num(raw?.fill_id ?? raw?.fillId),
    fillPrice: num(raw?.fill_price ?? raw?.fillPrice),
    volume: num(raw?.volume),
    costAmount: num(raw?.cost_amount ?? raw?.costAmount),
    tradeDate: raw?.trade_date || raw?.tradeDate ? str(raw?.trade_date ?? raw?.tradeDate) : undefined,
    strategyName: raw?.strategy_name || raw?.strategyName ? str(raw?.strategy_name ?? raw?.strategyName) : undefined,
  }
}

function mapAttributionRow(raw: any): AttributionPositionRow {
  const un = raw?.unattributed
  return {
    stockCode: str(raw?.stock_code ?? raw?.stockCode),
    stockName: str(raw?.stock_name ?? raw?.stockName),
    totalVolume: num(raw?.total_volume ?? raw?.totalVolume),
    currentPrice: num(raw?.current_price ?? raw?.currentPrice),
    pnl: num(raw?.pnl),
    lots: Array.isArray(raw?.lots) ? raw.lots.map(mapAttributionLot) : [],
    reconcile: {
      positionVolume: num(raw?.reconcile?.position_volume ?? raw?.reconcile?.positionVolume),
      attributedVolume: num(raw?.reconcile?.attributed_volume ?? raw?.reconcile?.attributedVolume),
      unattributedVolume: num(raw?.reconcile?.unattributed_volume ?? raw?.reconcile?.unattributedVolume),
      surplusFillVolume:
        raw?.reconcile?.surplus_fill_volume != null || raw?.reconcile?.surplusFillVolume != null
          ? num(raw?.reconcile?.surplus_fill_volume ?? raw?.reconcile?.surplusFillVolume)
          : undefined,
      status: str(raw?.reconcile?.status, 'matched'),
    },
    unattributed: un
      ? {
          volume: num(un.volume),
          reasonCode: str(un.reason_code ?? un.reasonCode),
          message: str(un.message),
        }
      : undefined,
    sourceSummary: raw?.source_summary || raw?.sourceSummary ? str(raw?.source_summary ?? raw?.sourceSummary) : undefined,
    planIds: Array.isArray(raw?.plan_ids)
      ? raw.plan_ids.map((x: unknown) => num(x))
      : Array.isArray(raw?.planIds)
        ? raw.planIds.map((x: unknown) => num(x))
        : undefined,
  }
}

/** GET /api/papertrading/observation/positions/attribution */
export async function getPaperPositionAttribution(stockCode?: string): Promise<PositionAttributionView> {
  const q = stockCode?.trim() ? `?stock_code=${encodeURIComponent(stockCode.trim())}` : ''
  const res = await fetch(`/api/papertrading/observation/positions/attribution${q}`)
  if (!res.ok) throw new Error(`持仓归因请求失败: HTTP ${res.status}`)
  const body = await res.json()
  if (!body?.ok) throw new Error(body?.message || '持仓归因响应无效')
  const raw = body.attribution || {}
  return {
    enabled: !!raw.enabled,
    accountId: raw.account_id || raw.accountId ? num(raw.account_id ?? raw.accountId) : undefined,
    asOf: raw.as_of || raw.asOf ? str(raw.as_of ?? raw.asOf) : undefined,
    reconcileAllMatched: !!(raw.reconcile_all_matched ?? raw.reconcileAllMatched),
    positions: Array.isArray(raw.positions) ? raw.positions.map(mapAttributionRow) : [],
    dataSourceNote: str(raw.data_source_note ?? raw.dataSourceNote),
  }
}

// --- Holding Evaluation (D.1.3 Observation) ---

export type HoldingEvalLot = {
  fillId: number
  planId: number
  planItemId: number
  buyDate?: string
  volume: number
  costPrice: number
  currentPrice: number | null
  quoteSource?: string
  quoteTime?: string
  pnl: number | null
  returnRate: number | null
  holdingDays: number
  evalState: string
  profitState?: string
  holdingPeriodState?: string
}

export type SecurityDisplayName = {
  snapshotName: string
  currentName: string
  displayName: string
  nameChanged: boolean
}

export type HoldingEvalStockRow = {
  stockCode: string
  stockName: string
  totalVolume: number
  avgCost: number | null
  marketPrice: number | null
  currentPrice: number | null
  quoteSource?: string
  quoteTime?: string
  marketValue: number | null
  unrealizedPnl: number | null
  unrealizedReturn: number | null
  holdingDays: number
  trendState: string
  riskState: string
  profitState?: string
  holdingPeriodState?: string
  evalState: string
  lots: HoldingEvalLot[]
  reconcileStatus?: string
  firstBuyDate?: string
  displayName?: SecurityDisplayName
}

export type HoldingEvalUnattributable = {
  stockCode: string
  stockName?: string
  volume: number
  reasonCode: string
  message?: string
}

export type HoldingsEvaluationView = {
  enabled: boolean
  accountId?: number
  asOf?: string
  reconcileAllMatched: boolean
  holdings: HoldingEvalStockRow[]
  unattributable: HoldingEvalUnattributable[]
  dataSourceNote: string
}

function nullableNum(v: unknown): number | null {
  if (v === null || v === undefined || v === '') return null
  const n = Number(v)
  return Number.isFinite(n) ? n : null
}

function mapHoldingEvalLot(raw: any): HoldingEvalLot {
  return {
    fillId: num(raw?.fill_id ?? raw?.fillId),
    planId: num(raw?.plan_id ?? raw?.planId),
    planItemId: num(raw?.plan_item_id ?? raw?.planItemId),
    buyDate: raw?.buy_date || raw?.buyDate ? str(raw?.buy_date ?? raw?.buyDate) : undefined,
    volume: num(raw?.volume),
    costPrice: num(raw?.cost_price ?? raw?.costPrice),
    currentPrice: nullableNum(raw?.current_price ?? raw?.currentPrice),
    quoteSource: raw?.quote_source || raw?.quoteSource ? str(raw?.quote_source ?? raw?.quoteSource) : undefined,
    quoteTime: raw?.quote_time || raw?.quoteTime ? str(raw?.quote_time ?? raw?.quoteTime) : undefined,
    pnl: nullableNum(raw?.pnl),
    returnRate: nullableNum(raw?.return_rate ?? raw?.returnRate),
    holdingDays: num(raw?.holding_days ?? raw?.holdingDays),
    evalState: str(raw?.eval_state ?? raw?.evalState, 'NORMAL'),
    profitState: str(raw?.profit_state ?? raw?.profitState, 'UNKNOWN'),
    holdingPeriodState: str(raw?.holding_period_state ?? raw?.holdingPeriodState, 'SHORT_TERM'),
  }
}

function mapDisplayName(raw: any, fallbackName: string): SecurityDisplayName {
  const d = raw || {}
  return {
    snapshotName: str(d.snapshot_name ?? d.snapshotName, fallbackName),
    currentName: str(d.current_name ?? d.currentName, 'UNKNOWN'),
    displayName: str(d.display_name ?? d.displayName, fallbackName || 'UNKNOWN'),
    nameChanged: !!(d.name_changed ?? d.nameChanged),
  }
}

function mapHoldingEvalStock(raw: any): HoldingEvalStockRow {
  const stockName = str(raw?.stock_name ?? raw?.stockName)
  return {
    stockCode: str(raw?.stock_code ?? raw?.stockCode),
    stockName,
    totalVolume: num(raw?.total_volume ?? raw?.totalVolume),
    avgCost: nullableNum(raw?.avg_cost ?? raw?.avgCost),
    marketPrice: nullableNum(raw?.market_price ?? raw?.marketPrice),
    currentPrice: nullableNum(raw?.current_price ?? raw?.currentPrice ?? raw?.market_price ?? raw?.marketPrice),
    quoteSource: raw?.quote_source || raw?.quoteSource ? str(raw?.quote_source ?? raw?.quoteSource) : undefined,
    quoteTime: raw?.quote_time || raw?.quoteTime ? str(raw?.quote_time ?? raw?.quoteTime) : undefined,
    marketValue: nullableNum(raw?.market_value ?? raw?.marketValue),
    unrealizedPnl: nullableNum(raw?.unrealized_pnl ?? raw?.unrealizedPnl),
    unrealizedReturn: nullableNum(raw?.unrealized_return ?? raw?.unrealizedReturn),
    holdingDays: num(raw?.holding_days ?? raw?.holdingDays),
    trendState: str(raw?.trend_state ?? raw?.trendState, 'UNKNOWN'),
    riskState: str(raw?.risk_state ?? raw?.riskState, 'NORMAL'),
    profitState: str(raw?.profit_state ?? raw?.profitState, 'UNKNOWN'),
    holdingPeriodState: str(raw?.holding_period_state ?? raw?.holdingPeriodState, 'SHORT_TERM'),
    evalState: str(raw?.eval_state ?? raw?.evalState, 'NORMAL'),
    lots: Array.isArray(raw?.lots) ? raw.lots.map(mapHoldingEvalLot) : [],
    reconcileStatus: raw?.reconcile_status || raw?.reconcileStatus
      ? str(raw?.reconcile_status ?? raw?.reconcileStatus)
      : undefined,
    firstBuyDate: raw?.first_buy_date || raw?.firstBuyDate
      ? str(raw?.first_buy_date ?? raw?.firstBuyDate)
      : undefined,
    displayName: mapDisplayName(raw?.display_name ?? raw?.displayName, stockName),
  }
}

/** GET /api/papertrading/observation/holdings/evaluation */
export async function getPaperHoldingsEvaluation(stockCode?: string): Promise<HoldingsEvaluationView> {
  const q = stockCode?.trim() ? `?stock_code=${encodeURIComponent(stockCode.trim())}` : ''
  const res = await fetch(`/api/papertrading/observation/holdings/evaluation${q}`)
  if (!res.ok) throw new Error(`持仓评价请求失败: HTTP ${res.status}`)
  const body = await res.json()
  if (!body?.ok) throw new Error(body?.message || '持仓评价响应无效')
  const raw = body.evaluation || {}
  return {
    enabled: !!raw.enabled,
    accountId: raw.account_id || raw.accountId ? num(raw.account_id ?? raw.accountId) : undefined,
    asOf: raw.as_of || raw.asOf ? str(raw.as_of ?? raw.asOf) : undefined,
    reconcileAllMatched: !!(raw.reconcile_all_matched ?? raw.reconcileAllMatched),
    holdings: Array.isArray(raw.holdings) ? raw.holdings.map(mapHoldingEvalStock) : [],
    unattributable: Array.isArray(raw.unattributable)
      ? raw.unattributable.map((u: any) => ({
          stockCode: str(u?.stock_code ?? u?.stockCode),
          stockName: u?.stock_name || u?.stockName ? str(u?.stock_name ?? u?.stockName) : undefined,
          volume: num(u?.volume),
          reasonCode: str(u?.reason_code ?? u?.reasonCode),
          message: u?.message ? str(u.message) : undefined,
        }))
      : [],
    dataSourceNote: str(raw.data_source_note ?? raw.dataSourceNote),
  }
}

// --- Exit Evaluation (D.2.1 Observation, read-only re-assessment) ---

export type ExitEvaluationLabel = {
  state: string
  reasonCodes: string[]
  summary?: string
}

export type ExitEntryContext = {
  strategyName: string
  entryReason: string
  entryRule: string
  intentStatus: string
}

export type ExitPlanContext = {
  planId: number
  tradeDate: string
  planStatus: string
}

export type ExitContext = {
  entry: ExitEntryContext
  plan: ExitPlanContext
}

export type ExitEvaluationLot = {
  fillId: number
  planId: number
  planItemId: number
  holdingDays: number
  unrealizedReturn: number | null
  evaluation: ExitEvaluationLabel
  context: ExitContext
}

export type ExitReviewOutcomeSummary = {
  id: string
  decision: string
  reviewTime: string
  reason?: string
  createdBy: string
  relatedTradePlanId?: number
}

/** Phase17-C2 explanation (observation only). */
export type PositionEvaluationExplanation = {
  stockCode: string
  evaluationTime?: string
  positionDays: number
  costPrice: number | null
  currentPrice: number | null
  pnl: number | null
  sourceType: string
  signalContext: {
    signalSnapshotId?: number
    signalTime?: string
    signalPrice?: number
    signalTag?: string
    present: boolean
  }
  holdReasons: string[]
  riskHints: string[]
  freshness?: {
    priceStatus?: string
    klineStatus?: string
    status?: string
    priceAge?: string
    klineAge?: string
  }
  dataSourceNote?: string
}

/** Phase17-C3 health score (quality only; not a sell signal). */
export type HoldingHealthScore = {
  stockCode: string
  evaluationTime?: string
  score: number
  grade: string
  gradeLabel: string
  supportingFactors: string[]
  riskFactors: string[]
  explanation?: PositionEvaluationExplanation
  dataSourceNote?: string
}

/** Phase17.1 做 T suitability (operability only; not a trade signal). */
export type HoldingTSuitability = {
  stockCode: string
  level: string
  reasons: string[]
  canSell: boolean
  freshness: string
  volatilityStatus: string
  healthGrade: string
  evaluatedAt?: string
  dataSourceNote?: string
}

export type ExitEvaluationStockRow = {
  stockCode: string
  stockName: string
  evaluation: ExitEvaluationLabel
  lots: ExitEvaluationLot[]
  latestOutcome?: ExitReviewOutcomeSummary
  explanation?: PositionEvaluationExplanation
  healthScore?: HoldingHealthScore
  tSuitability?: HoldingTSuitability
}

export type ExitEvaluationView = {
  enabled: boolean
  accountId?: number
  asOf?: string
  holdings: ExitEvaluationStockRow[]
  dataSourceNote: string
  policyNote?: string
  policy?: {
    policyId: string
    version: number
    maxHoldingDays: number
    lossWatchThreshold: number
    lossReviewThreshold: number
  }
}

function mapExitEvalLabel(raw: any): ExitEvaluationLabel {
  const codes = raw?.reason_codes ?? raw?.reasonCodes
  return {
    state: str(raw?.state, 'NORMAL'),
    reasonCodes: Array.isArray(codes) ? codes.map((c: unknown) => str(c)) : [],
    summary: raw?.summary ? str(raw.summary) : undefined,
  }
}

function mapExitContext(raw: any): ExitContext {
  const entry = raw?.entry || {}
  const plan = raw?.plan || {}
  return {
    entry: {
      strategyName: str(entry?.strategy_name ?? entry?.strategyName),
      entryReason: str(entry?.entry_reason ?? entry?.entryReason),
      entryRule: str(entry?.entry_rule ?? entry?.entryRule),
      intentStatus: str(entry?.intent_status ?? entry?.intentStatus),
    },
    plan: {
      planId: num(plan?.plan_id ?? plan?.planId),
      tradeDate: str(plan?.trade_date ?? plan?.tradeDate),
      planStatus: str(plan?.plan_status ?? plan?.planStatus),
    },
  }
}

function mapExitEvalLot(raw: any): ExitEvaluationLot {
  return {
    fillId: num(raw?.fill_id ?? raw?.fillId),
    planId: num(raw?.plan_id ?? raw?.planId),
    planItemId: num(raw?.plan_item_id ?? raw?.planItemId),
    holdingDays: num(raw?.holding_days ?? raw?.holdingDays),
    unrealizedReturn: nullableNum(raw?.unrealized_return ?? raw?.unrealizedReturn),
    evaluation: mapExitEvalLabel(raw?.evaluation),
    context: mapExitContext(raw?.context),
  }
}

function mapExitOutcomeSummary(raw: any): ExitReviewOutcomeSummary | undefined {
  if (!raw || typeof raw !== 'object') return undefined
  const id = str(raw?.id)
  if (!id) return undefined
  return {
    id,
    decision: str(raw?.decision),
    reviewTime: str(raw?.review_time ?? raw?.reviewTime),
    reason: raw?.reason ? str(raw.reason) : undefined,
    createdBy: str(raw?.created_by ?? raw?.createdBy, 'ui:exit-review'),
    relatedTradePlanId:
      raw?.related_trade_plan_id != null || raw?.relatedTradePlanId != null
        ? num(raw?.related_trade_plan_id ?? raw?.relatedTradePlanId)
        : undefined,
  }
}

function mapPositionExplanation(raw: any): PositionEvaluationExplanation | undefined {
  if (!raw || typeof raw !== 'object') return undefined
  const sig = raw.signal_context ?? raw.signalContext ?? {}
  const fresh = raw.freshness ?? {}
  return {
    stockCode: str(raw.stock_code ?? raw.stockCode),
    evaluationTime: raw.evaluation_time || raw.evaluationTime
      ? str(raw.evaluation_time ?? raw.evaluationTime)
      : undefined,
    positionDays: num(raw.position_days ?? raw.positionDays),
    costPrice: nullableNum(raw.cost_price ?? raw.costPrice),
    currentPrice: nullableNum(raw.current_price ?? raw.currentPrice),
    pnl: nullableNum(raw.pnl),
    sourceType: str(raw.source_type ?? raw.sourceType, 'Unknown'),
    signalContext: {
      signalSnapshotId:
        raw.signal_snapshot_id != null || sig.signal_snapshot_id != null || sig.signalSnapshotId != null
          ? num(raw.signal_snapshot_id ?? sig.signal_snapshot_id ?? sig.signalSnapshotId)
          : undefined,
      signalTime: str(sig.signal_time ?? sig.signalTime ?? raw.signal_time ?? ''),
      signalPrice: nullableNum(sig.signal_price ?? sig.signalPrice) ?? undefined,
      signalTag: str(sig.signal_tag ?? sig.signalTag ?? ''),
      present: !!(sig.present ?? raw.signal_present),
    },
    holdReasons: Array.isArray(raw.hold_reasons ?? raw.holdReasons)
      ? (raw.hold_reasons ?? raw.holdReasons).map((x: any) => String(x))
      : [],
    riskHints: Array.isArray(raw.risk_hints ?? raw.riskHints)
      ? (raw.risk_hints ?? raw.riskHints).map((x: any) => String(x))
      : [],
    freshness: {
      priceStatus: str(fresh.price_status ?? fresh.priceStatus),
      klineStatus: str(fresh.kline_status ?? fresh.klineStatus),
      status: str(fresh.status),
      priceAge: str(fresh.price_age ?? fresh.priceAge),
      klineAge: str(fresh.kline_age ?? fresh.klineAge),
    },
    dataSourceNote: raw.data_source_note || raw.dataSourceNote
      ? str(raw.data_source_note ?? raw.dataSourceNote)
      : undefined,
  }
}

function mapHoldingHealthScore(raw: any): HoldingHealthScore | undefined {
  if (!raw || typeof raw !== 'object') return undefined
  const grade = str(raw.grade)
  if (!grade && raw.score == null) return undefined
  const support = raw.supporting_factors ?? raw.supportingFactors ?? raw.factors
  const risks = raw.risk_factors ?? raw.riskFactors
  return {
    stockCode: str(raw.stock_code ?? raw.stockCode),
    evaluationTime: raw.evaluation_time || raw.evaluationTime
      ? str(raw.evaluation_time ?? raw.evaluationTime)
      : undefined,
    score: num(raw.score),
    grade,
    gradeLabel: str(raw.grade_label ?? raw.gradeLabel),
    supportingFactors: Array.isArray(support) ? support.map((x: any) => String(x)) : [],
    riskFactors: Array.isArray(risks) ? risks.map((x: any) => String(x)) : [],
    explanation: mapPositionExplanation(raw.explanation),
    dataSourceNote: raw.data_source_note || raw.dataSourceNote
      ? str(raw.data_source_note ?? raw.dataSourceNote)
      : undefined,
  }
}

function mapHoldingTSuitability(raw: any): HoldingTSuitability | undefined {
  if (!raw || typeof raw !== 'object') return undefined
  const level = str(raw.level)
  if (!level) return undefined
  const reasons = raw.reasons
  return {
    stockCode: str(raw.stock_code ?? raw.stockCode),
    level,
    reasons: Array.isArray(reasons) ? reasons.map((x: any) => String(x)) : [],
    canSell: !!(raw.can_sell ?? raw.canSell),
    freshness: str(raw.freshness),
    volatilityStatus: str(raw.volatility_status ?? raw.volatilityStatus),
    healthGrade: str(raw.health_grade ?? raw.healthGrade),
    evaluatedAt: raw.evaluated_at || raw.evaluatedAt
      ? str(raw.evaluated_at ?? raw.evaluatedAt)
      : undefined,
    dataSourceNote: raw.data_source_note || raw.dataSourceNote
      ? str(raw.data_source_note ?? raw.dataSourceNote)
      : undefined,
  }
}

function mapExitEvalStock(raw: any): ExitEvaluationStockRow {
  return {
    stockCode: str(raw?.stock_code ?? raw?.stockCode),
    stockName: str(raw?.stock_name ?? raw?.stockName),
    evaluation: mapExitEvalLabel(raw?.evaluation),
    lots: Array.isArray(raw?.lots) ? raw.lots.map(mapExitEvalLot) : [],
    latestOutcome: mapExitOutcomeSummary(raw?.latest_outcome ?? raw?.latestOutcome),
    explanation: mapPositionExplanation(raw?.explanation),
    healthScore: mapHoldingHealthScore(raw?.health_score ?? raw?.healthScore),
    tSuitability: mapHoldingTSuitability(raw?.t_suitability ?? raw?.tSuitability),
  }
}

/** GET /api/papertrading/observation/holdings/exit-evaluation */
export async function getPaperExitEvaluation(stockCode?: string): Promise<ExitEvaluationView> {
  const q = stockCode?.trim() ? `?stock_code=${encodeURIComponent(stockCode.trim())}` : ''
  const res = await fetch(`/api/papertrading/observation/holdings/exit-evaluation${q}`)
  if (!res.ok) throw new Error(`退出评估请求失败: HTTP ${res.status}`)
  const body = await res.json()
  if (!body?.ok) throw new Error(body?.message || '退出评估响应无效')
  const raw = body.exit_evaluation || body.exitEvaluation || {}
  const pol = raw.policy || {}
  return {
    enabled: !!raw.enabled,
    accountId: raw.account_id || raw.accountId ? num(raw.account_id ?? raw.accountId) : undefined,
    asOf: raw.as_of || raw.asOf ? str(raw.as_of ?? raw.asOf) : undefined,
    holdings: Array.isArray(raw.holdings) ? raw.holdings.map(mapExitEvalStock) : [],
    dataSourceNote: str(raw.data_source_note ?? raw.dataSourceNote),
    policyNote: raw.policy_note || raw.policyNote ? str(raw.policy_note ?? raw.policyNote) : undefined,
    policy: raw.policy
      ? {
          policyId: str(pol.policy_id ?? pol.policyId),
          version: num(pol.version),
          maxHoldingDays: num(pol.max_holding_days ?? pol.maxHoldingDays),
          lossWatchThreshold: num(pol.loss_watch_threshold ?? pol.lossWatchThreshold),
          lossReviewThreshold: num(pol.loss_review_threshold ?? pol.lossReviewThreshold),
        }
      : undefined,
  }
}

export type SaveExitReviewOutcomeRequest = {
  stockCode: string
  decision: 'HOLD' | 'WATCH' | 'CREATE_SELL_PLAN'
  reason?: string
  reviewTime?: string
  createdBy?: string
  exitStateSnapshot: string
  reasonCodesSnapshot?: string[]
  evaluationSummarySnapshot?: string
  relatedTradePlanId?: number
  accountId?: number
}

export type SaveExitReviewOutcomeResponse = {
  id: string
  decision: string
  reviewTime: string
  reason?: string
  createdBy: string
  relatedTradePlanId?: number
}

/** POST /api/exit-review/outcome */
export async function postExitReviewOutcome(
  req: SaveExitReviewOutcomeRequest,
): Promise<SaveExitReviewOutcomeResponse> {
  const res = await fetch('/api/exit-review/outcome', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      stock_code: req.stockCode,
      decision: req.decision,
      reason: req.reason || '',
      review_time: req.reviewTime || new Date().toISOString(),
      created_by: req.createdBy || 'ui:exit-review',
      exit_state_snapshot: req.exitStateSnapshot,
      reason_codes_snapshot: req.reasonCodesSnapshot || [],
      evaluation_summary_snapshot: req.evaluationSummarySnapshot || '',
      related_trade_plan_id: req.relatedTradePlanId ?? null,
      account_id: req.accountId || 0,
    }),
  })
  if (!res.ok) throw new Error(`保存复评结论失败: HTTP ${res.status}`)
  const body = await res.json()
  if (!body?.ok) throw new Error(body?.message || '保存复评结论响应无效')
  const raw = body.outcome || {}
  return {
    id: str(raw?.id),
    decision: str(raw?.decision),
    reviewTime: str(raw?.review_time ?? raw?.reviewTime),
    reason: raw?.reason ? str(raw.reason) : undefined,
    createdBy: str(raw?.created_by ?? raw?.createdBy),
    relatedTradePlanId:
      raw?.related_trade_plan_id != null ? num(raw.related_trade_plan_id) : undefined,
  }
}


/** GET /api/papertrading/observation/execution-summary */
export type ExecutionSummaryView = {
  enabled: boolean
  tradeDate?: string
  totalOrders: number
  filledOrders: number
  failedOrders: number
  fillRate: number
  avgSlippage: number | null
  dataSourceNote: string
}

export async function getPaperExecutionSummary(tradeDate?: string): Promise<ExecutionSummaryView> {
  const q = tradeDate?.trim() ? `?trade_date=${encodeURIComponent(tradeDate.trim())}` : ''
  const res = await fetch(`/api/papertrading/observation/execution-summary${q}`)
  if (!res.ok) throw new Error(`Execution Summary 请求失败: HTTP ${res.status}`)
  const body = await res.json()
  if (!body?.ok) throw new Error(body?.message || 'Execution Summary 无效')
  const raw = body.execution_summary || body.executionSummary || {}
  return {
    enabled: !!raw.enabled,
    tradeDate: raw.trade_date || raw.tradeDate ? str(raw.trade_date ?? raw.tradeDate) : undefined,
    totalOrders: num(raw.total_orders ?? raw.totalOrders),
    filledOrders: num(raw.filled_orders ?? raw.filledOrders),
    failedOrders: num(raw.failed_orders ?? raw.failedOrders),
    fillRate: num(raw.fill_rate ?? raw.fillRate),
    avgSlippage: nullableNum(raw.avg_slippage ?? raw.avgSlippage),
    dataSourceNote: str(raw.data_source_note ?? raw.dataSourceNote),
  }
}

/** GET /api/papertrading/observation/risk */
export type RiskObservationView = {
  enabled: boolean
  quality: string
  totalAsset: number | null
  cash: number | null
  positionValue: number | null
  positionRatio: number | null
  concentration: number | null
  positions: Array<{
    stockCode: string
    stockName: string
    marketValue: number
    singleStockWeight: number | null
  }>
  dataSourceNote: string
}

export async function getPaperRiskObservation(): Promise<RiskObservationView> {
  const res = await fetch('/api/papertrading/observation/risk')
  if (!res.ok) throw new Error(`Risk Observation 请求失败: HTTP ${res.status}`)
  const body = await res.json()
  if (!body?.ok) throw new Error(body?.message || 'Risk Observation 无效')
  const raw = body.risk || {}
  const positions = Array.isArray(raw.positions) ? raw.positions : []
  return {
    enabled: !!raw.enabled,
    quality: str(raw.quality, 'UNKNOWN'),
    totalAsset: nullableNum(raw.total_asset ?? raw.totalAsset),
    cash: nullableNum(raw.cash),
    positionValue: nullableNum(raw.position_value ?? raw.positionValue),
    positionRatio: nullableNum(raw.position_ratio ?? raw.positionRatio),
    concentration: nullableNum(raw.concentration),
    positions: positions.map((p: any) => ({
      stockCode: str(p?.stock_code ?? p?.stockCode),
      stockName: str(p?.stock_name ?? p?.stockName),
      marketValue: num(p?.market_value ?? p?.marketValue),
      singleStockWeight: nullableNum(p?.single_stock_weight ?? p?.singleStockWeight),
    })),
    dataSourceNote: str(raw.data_source_note ?? raw.dataSourceNote),
  }
}
