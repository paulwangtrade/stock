/** Phase13 Portfolio Decision Dashboard — read-only GET client */

function num(v: unknown, d = NaN): number {
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

function bool(v: unknown, d = false): boolean {
  if (v === true || v === false) return v
  return d
}

export type ExplainFactor = {
  code: string
  plainText: string
  available: boolean
  source: string
}

export type ReduceSuggestionRow = {
  symbol: string
  action: string
  reason: string
  suggestSellQty: number
  targetWeight: number
  riskReason: string
}

export type SectorBucket = {
  sector: string
  weight: number
  nameCount: number
}

export type PortfolioDecisionDashboardView = {
  schemaVersion: string
  asOf: string
  tradeDate: string
  accountId: string
  disclaimer: string
  readOnly: boolean
  notTradingAdvice: boolean
  notAutoTrade: boolean
  notATradePlan: boolean
  notExecution: boolean
  notProviderSwitch: boolean
  analysisToolOnly: boolean
  currentPortfolio: {
    available: boolean
    note: string
    equity: number | null
    nameCount: number | null
    cashRatio: number | null
    top1Weight: number | null
    top5Weight: number | null
    grossExposure: number | null
    sectorAvailable: boolean
    sectorNote: string
    maxSectorWeight: number | null
    allowSectorConstraint: boolean | null
    sectorBuckets: SectorBucket[]
    riskLevel: string
    riskLevelLabel: string
    riskReasons: string[]
  }
  decisionExplain: {
    whyBuyLess: ExplainFactor[]
    whyPositionLimited: ExplainFactor[]
    whySuggestReduce: ExplainFactor[]
    reduceRows: ReduceSuggestionRow[]
  }
  sectorCoverage: {
    present: boolean
    holdingsCoverage: number
    poolCoverage: number
    unknownCount: number
    allowSectorConstraint: boolean
    missingCount: number
    note: string
    /** derived: available when present && allow; else unavailable */
    status: 'available' | 'unavailable'
  }
  insight: {
    present: boolean
    riskLevel: string
    riskLevelLabel: string
    riskReasons: string[]
    nameCount: number | null
    cashRatio: number | null
    dataGaps: string[]
  }
  validation: {
    present: boolean
    skipped: boolean
    note: string
    dayCount: number
    okCount: number
  }
  dataGaps: string[]
  warnings: string[]
  dataSourceNote: string
  flags: {
    readOnly: boolean
    notAutoTrade: boolean
    notATradePlan: boolean
    notOrder: boolean
    notExecution: boolean
    notProviderSwitch: boolean
  }
}

function mapFactor(raw: any): ExplainFactor {
  return {
    code: str(raw?.code),
    plainText: str(raw?.plain_text ?? raw?.plainText),
    available: bool(raw?.available, true),
    source: str(raw?.source),
  }
}

function mapView(body: any): PortfolioDecisionDashboardView {
  const obs = body?.observation || {}
  const cur = obs.current_portfolio || obs.currentPortfolio || {}
  const ex = cur.exposure || {}
  const conc = cur.concentration || {}
  const sector = cur.sector || {}
  const cash = cur.cash || {}
  const explain = obs.decision_explain || obs.decisionExplain || {}
  const cov = obs.sector_coverage || obs.sectorCoverage || {}
  const flags = body?.flags || {}
  const allow =
    sector.allow_sector_constraint ?? sector.allowSectorConstraint
  const allowBool =
    allow === true || allow === false ? (allow as boolean) : null
  const covAllow = bool(cov.allow_sector_constraint ?? cov.allowSectorConstraint, false)
  const covPresent = bool(cov.present, false)
  const bucketsRaw = sector.buckets || []
  const gaps = Array.isArray(obs.data_gaps)
    ? obs.data_gaps.map((g: unknown) => str(g))
    : Array.isArray(obs.dataGaps)
      ? obs.dataGaps.map((g: unknown) => str(g))
      : []

  const riskLevel = str(cur.risk_level ?? cur.riskLevel)
  const riskLabel = str(cur.risk_level_label ?? cur.riskLevelLabel)
  const riskReasons = Array.isArray(cur.risk_reasons ?? cur.riskReasons)
    ? (cur.risk_reasons ?? cur.riskReasons).map((r: unknown) => str(r))
    : []

  return {
    schemaVersion: str(obs.schema_version ?? obs.schemaVersion),
    asOf: str(obs.as_of ?? obs.asOf),
    tradeDate: str(obs.trade_date ?? obs.tradeDate),
    accountId: str(obs.account_id ?? obs.accountId),
    disclaimer: str(
      obs.disclaimer,
      '本页为组合观察与决策过程说明，不构成投资建议，不会自动买卖。',
    ),
    readOnly: bool(obs.read_only ?? obs.readOnly, true),
    notTradingAdvice: bool(obs.not_trading_advice ?? obs.notTradingAdvice, true),
    notAutoTrade: bool(obs.not_auto_trade ?? obs.notAutoTrade ?? flags.not_auto_trade, true),
    notATradePlan: bool(obs.not_a_trade_plan ?? obs.notATradePlan ?? flags.not_a_trade_plan, true),
    notExecution: bool(obs.not_execution ?? obs.notExecution ?? flags.not_execution, true),
    notProviderSwitch: bool(
      obs.not_provider_switch ?? obs.notProviderSwitch ?? flags.not_provider_switch,
      true,
    ),
    analysisToolOnly: bool(obs.analysis_tool_only ?? obs.analysisToolOnly, true),
    currentPortfolio: {
      available: bool(cur.available, false),
      note: str(cur.note),
      equity: nullableNum(ex.equity),
      nameCount:
        cur.name_count != null || cur.nameCount != null
          ? num(cur.name_count ?? cur.nameCount, 0)
          : conc.name_count != null || conc.nameCount != null
            ? num(conc.name_count ?? conc.nameCount, 0)
            : null,
      cashRatio: nullableNum(cash.cash_ratio ?? cash.cashRatio),
      top1Weight: nullableNum(conc.top1_weight ?? conc.top1Weight),
      top5Weight: nullableNum(conc.top5_weight ?? conc.top5Weight),
      grossExposure: nullableNum(ex.gross_exposure ?? ex.grossExposure),
      sectorAvailable: bool(sector.available, false),
      sectorNote: str(sector.note || sector.coverage_note || sector.coverageNote),
      maxSectorWeight: nullableNum(sector.max_sector_weight ?? sector.maxSectorWeight),
      allowSectorConstraint: allowBool,
      sectorBuckets: (Array.isArray(bucketsRaw) ? bucketsRaw : []).map((b: any) => ({
        sector: str(b?.sector),
        weight: num(b?.weight, 0),
        nameCount: num(b?.name_count ?? b?.nameCount, 0),
      })),
      riskLevel,
      riskLevelLabel: riskLabel,
      riskReasons,
    },
    decisionExplain: {
      whyBuyLess: (explain.why_buy_less || explain.whyBuyLess || []).map(mapFactor),
      whyPositionLimited: (explain.why_position_limited || explain.whyPositionLimited || []).map(
        mapFactor,
      ),
      whySuggestReduce: (explain.why_suggest_reduce || explain.whySuggestReduce || []).map(
        mapFactor,
      ),
      reduceRows: (explain.reduce_rows || explain.reduceRows || []).map((r: any) => ({
        symbol: str(r?.symbol),
        action: str(r?.action),
        reason: str(r?.reason),
        suggestSellQty: num(r?.suggest_sell_qty ?? r?.suggestSellQty, 0),
        targetWeight: num(r?.target_weight ?? r?.targetWeight, 0),
        riskReason: str(r?.risk_reason ?? r?.riskReason),
      })),
    },
    sectorCoverage: {
      present: covPresent,
      holdingsCoverage: num(cov.holdings_coverage ?? cov.holdingsCoverage, 0),
      poolCoverage: num(cov.pool_coverage ?? cov.poolCoverage, 0),
      unknownCount: num(cov.unknown_count ?? cov.unknownCount, 0),
      allowSectorConstraint: covAllow,
      missingCount: num(cov.missing_count ?? cov.missingCount, 0),
      note: str(cov.note),
      status: covPresent && covAllow ? 'available' : 'unavailable',
    },
    insight: {
      present: bool(obs.sources?.insight_present ?? obs.sources?.insightPresent, !!riskLevel),
      riskLevel,
      riskLevelLabel: riskLabel,
      riskReasons,
      nameCount:
        cur.name_count != null || cur.nameCount != null
          ? num(cur.name_count ?? cur.nameCount, 0)
          : null,
      cashRatio: nullableNum(cash.cash_ratio ?? cash.cashRatio),
      dataGaps: gaps.filter((g) => g.includes('insight') || g.includes('why_')),
    },
    validation: {
      present: bool(
        (obs.validation || {}).present,
        false,
      ),
      skipped: bool((obs.validation || {}).skipped, false),
      note: str((obs.validation || {}).note),
      dayCount: num((obs.validation || {}).day_count ?? (obs.validation || {}).dayCount, 0),
      okCount: num((obs.validation || {}).ok_count ?? (obs.validation || {}).okCount, 0),
    },
    dataGaps: gaps,
    warnings: Array.isArray(obs.warnings) ? obs.warnings.map((w: unknown) => str(w)) : [],
    dataSourceNote: str(obs.data_source_note ?? obs.dataSourceNote),
    flags: {
      readOnly: bool(flags.read_only ?? flags.readOnly, true),
      notAutoTrade: bool(flags.not_auto_trade ?? flags.notAutoTrade, true),
      notATradePlan: bool(flags.not_a_trade_plan ?? flags.notATradePlan, true),
      notOrder: bool(flags.not_order ?? flags.notOrder, true),
      notExecution: bool(flags.not_execution ?? flags.notExecution, true),
      notProviderSwitch: bool(flags.not_provider_switch ?? flags.notProviderSwitch, true),
    },
  }
}

/** GET /api/papertrading/observation/portfolio-v2 — read-only; never TradePlan / Execution. */
export async function getPortfolioDecisionDashboard(query?: {
  tradeDate?: string
  accountId?: string
}): Promise<PortfolioDecisionDashboardView> {
  const params = new URLSearchParams()
  if (query?.tradeDate?.trim()) params.set('trade_date', query.tradeDate.trim())
  if (query?.accountId?.trim()) params.set('account_id', query.accountId.trim())
  const q = params.toString() ? `?${params.toString()}` : ''
  const res = await fetch(`/api/papertrading/observation/portfolio-v2${q}`)
  if (!res.ok) throw new Error(`决策看板请求失败: HTTP ${res.status}`)
  const body = await res.json()
  if (!body?.ok) throw new Error(body?.message || '决策看板响应无效')
  return mapView(body)
}
