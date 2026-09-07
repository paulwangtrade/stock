/**
 * Phase16-C2.0 — GET /api/opportunities/projections read-only client.
 * JSON field names match backend OpportunityProjection (snake_case).
 */

export const DECISION_STATUS = {
  BUY_CANDIDATE: 'BUY_CANDIDATE',
  WATCH: 'WATCH',
  REJECT: 'REJECT',
  NOT_IN_PLAN: 'NOT_IN_PLAN',
  UNKNOWN: 'UNKNOWN',
} as const

export type OpportunityProjectionSignal = {
  present: boolean
  snapshot_id?: number
  session?: string
  strategy_id?: string
  signal_tag?: string
  signal_price?: number
  signal_time?: string
  signal_status?: string
  trigger_reason?: string
  schema_version?: string
}

export type OpportunityProjectionOpportunity = {
  present: boolean
  pool_id?: number
  score?: number
  rank?: number
  strategy_source?: string
  strategy_name?: string
  pool_source?: string
}

export type OpportunityProjectionDecision = {
  decision_status: string
  candidate_status: string
  plan_id?: number
  plan_item_id?: number
  item_status?: string
  risk_code?: string
  target_amount?: number
  decision_provider?: string
}

export type OpportunityProjectionTradePlan = {
  present: boolean
  plan_id?: number
  plan_status?: string
  item_status?: string
  trade_date?: string
  frozen: boolean
}

export type OpportunityProjectionPortfolio = {
  holding_status: string
  position_qty?: number
}

export type OpportunityProjectionResearch = {
  research_id: string
  status?: string
  tags?: string[]
  signal_score?: number
  explain_summary?: string
}

export type OpportunityProjectionMetadata = {
  source_type: string
  quality: string
  missing?: string[]
  updated_at: string
}

/** Unified read model; field names align with backend JSON tags. */
export type OpportunityProjection = {
  opportunity_id: string
  stock_code: string
  stock_name?: string
  trade_date: string
  signal: OpportunityProjectionSignal
  opportunity: OpportunityProjectionOpportunity
  decision: OpportunityProjectionDecision
  trade_plan: OpportunityProjectionTradePlan
  portfolio: OpportunityProjectionPortfolio
  research?: OpportunityProjectionResearch
  metadata: OpportunityProjectionMetadata
}

export type OpportunityProjectionQuery = {
  tradeDate?: string
  limit?: number
  status?: string
  strategyId?: string
}

export type OpportunityProjectionsResult = {
  items: OpportunityProjection[]
  total: number
  generated_at: string
}

export class OpportunityProjectionNotFoundError extends Error {
  readonly status = 404

  constructor(message = 'projection not found') {
    super(message)
    this.name = 'OpportunityProjectionNotFoundError'
  }
}

function num(v: unknown): number | undefined {
  const n = Number(v)
  return Number.isFinite(n) ? n : undefined
}

function str(v: unknown): string {
  if (v == null) return ''
  return String(v)
}

function bool(v: unknown, fallback = false): boolean {
  if (typeof v === 'boolean') return v
  return fallback
}

function mapSignal(raw: Record<string, unknown> | undefined): OpportunityProjectionSignal {
  const r = raw && typeof raw === 'object' ? raw : {}
  const snapshotId = num(r.snapshot_id ?? r.snapshotId)
  const signalPrice = num(r.signal_price ?? r.signalPrice)
  return {
    present: bool(r.present),
    snapshot_id: snapshotId && snapshotId > 0 ? snapshotId : undefined,
    session: str(r.session) || undefined,
    strategy_id: str(r.strategy_id ?? r.strategyId) || undefined,
    signal_tag: str(r.signal_tag ?? r.signalTag) || undefined,
    signal_price: signalPrice,
    signal_time: str(r.signal_time ?? r.signalTime) || undefined,
    signal_status: str(r.signal_status ?? r.signalStatus) || undefined,
    trigger_reason: str(r.trigger_reason ?? r.triggerReason) || undefined,
    schema_version: str(r.schema_version ?? r.schemaVersion) || undefined,
  }
}

function mapOpportunityBlock(raw: Record<string, unknown> | undefined): OpportunityProjectionOpportunity {
  const r = raw && typeof raw === 'object' ? raw : {}
  const poolId = num(r.pool_id ?? r.poolId)
  const score = num(r.score)
  const rank = num(r.rank)
  return {
    present: bool(r.present),
    pool_id: poolId && poolId > 0 ? poolId : undefined,
    score,
    rank: rank != null ? Math.trunc(rank) : undefined,
    strategy_source: str(r.strategy_source ?? r.strategySource) || undefined,
    strategy_name: str(r.strategy_name ?? r.strategyName) || undefined,
    pool_source: str(r.pool_source ?? r.poolSource) || undefined,
  }
}

function mapDecision(raw: Record<string, unknown> | undefined): OpportunityProjectionDecision {
  const r = raw && typeof raw === 'object' ? raw : {}
  const planId = num(r.plan_id ?? r.planId)
  const planItemId = num(r.plan_item_id ?? r.planItemId)
  const targetAmount = num(r.target_amount ?? r.targetAmount)
  return {
    decision_status: str(r.decision_status ?? r.decisionStatus),
    candidate_status: str(r.candidate_status ?? r.candidateStatus),
    plan_id: planId && planId > 0 ? planId : undefined,
    plan_item_id: planItemId && planItemId > 0 ? planItemId : undefined,
    item_status: str(r.item_status ?? r.itemStatus) || undefined,
    risk_code: str(r.risk_code ?? r.riskCode) || undefined,
    target_amount: targetAmount,
    decision_provider: str(r.decision_provider ?? r.decisionProvider) || undefined,
  }
}

function mapTradePlan(raw: Record<string, unknown> | undefined): OpportunityProjectionTradePlan {
  const r = raw && typeof raw === 'object' ? raw : {}
  const planId = num(r.plan_id ?? r.planId)
  return {
    present: bool(r.present),
    plan_id: planId && planId > 0 ? planId : undefined,
    plan_status: str(r.plan_status ?? r.planStatus) || undefined,
    item_status: str(r.item_status ?? r.itemStatus) || undefined,
    trade_date: str(r.trade_date ?? r.tradeDate) || undefined,
    frozen: bool(r.frozen),
  }
}

function mapPortfolio(raw: Record<string, unknown> | undefined): OpportunityProjectionPortfolio {
  const r = raw && typeof raw === 'object' ? raw : {}
  const positionQty = num(r.position_qty ?? r.positionQty)
  return {
    holding_status: str(r.holding_status ?? r.holdingStatus),
    position_qty: positionQty,
  }
}

function mapResearch(raw: Record<string, unknown> | undefined): OpportunityProjectionResearch | undefined {
  if (!raw || typeof raw !== 'object') return undefined
  const researchId = str(raw.research_id ?? raw.researchId)
  if (!researchId) return undefined
  const signalScore = num(raw.signal_score ?? raw.signalScore)
  const tagsRaw = raw.tags
  const tags = Array.isArray(tagsRaw) ? tagsRaw.map((t) => String(t)) : undefined
  return {
    research_id: researchId,
    status: str(raw.status) || undefined,
    tags: tags?.length ? tags : undefined,
    signal_score: signalScore,
    explain_summary: str(raw.explain_summary ?? raw.explainSummary) || undefined,
  }
}

function mapMetadata(raw: Record<string, unknown> | undefined): OpportunityProjectionMetadata {
  const r = raw && typeof raw === 'object' ? raw : {}
  const missingRaw = r.missing
  const missing = Array.isArray(missingRaw) ? missingRaw.map((m) => String(m)) : undefined
  return {
    source_type: str(r.source_type ?? r.sourceType),
    quality: str(r.quality),
    missing: missing?.length ? missing : undefined,
    updated_at: str(r.updated_at ?? r.updatedAt),
  }
}

/** Map one backend OpportunityProjection JSON object. */
export function mapOpportunityProjection(raw: Record<string, unknown>): OpportunityProjection {
  return {
    opportunity_id: str(raw.opportunity_id ?? raw.opportunityId),
    stock_code: str(raw.stock_code ?? raw.stockCode),
    stock_name: str(raw.stock_name ?? raw.stockName) || undefined,
    trade_date: str(raw.trade_date ?? raw.tradeDate),
    signal: mapSignal(raw.signal as Record<string, unknown> | undefined),
    opportunity: mapOpportunityBlock(raw.opportunity as Record<string, unknown> | undefined),
    decision: mapDecision(raw.decision as Record<string, unknown> | undefined),
    trade_plan: mapTradePlan(raw.trade_plan as Record<string, unknown> | undefined),
    portfolio: mapPortfolio(raw.portfolio as Record<string, unknown> | undefined),
    research: mapResearch(raw.research as Record<string, unknown> | undefined),
    metadata: mapMetadata(raw.metadata as Record<string, unknown> | undefined),
  }
}

function buildSearchParams(q: OpportunityProjectionQuery & { stockCode?: string }): URLSearchParams {
  const params = new URLSearchParams()
  if (q.tradeDate?.trim()) params.set('trade_date', q.tradeDate.trim())
  if (q.stockCode?.trim()) params.set('stock_code', q.stockCode.trim())
  if (q.limit != null && q.limit > 0) params.set('limit', String(q.limit))
  if (q.status?.trim()) params.set('status', q.status.trim())
  if (q.strategyId?.trim()) params.set('strategy_id', q.strategyId.trim())
  return params
}

async function fetchProjectionsEnvelope(params: URLSearchParams): Promise<OpportunityProjectionsResult> {
  const res = await fetch(`/api/opportunities/projections?${params.toString()}`)
  const body = (await res.json().catch(() => ({}))) as Record<string, unknown>
  if (res.status === 404 || (body && body.ok === false && Number(body.code) === 404)) {
    throw new OpportunityProjectionNotFoundError(str(body?.message) || 'projection not found')
  }
  if (!res.ok) {
    throw new Error(str(body?.message) || `机会投影请求失败: HTTP ${res.status}`)
  }
  if (!body?.ok) {
    throw new Error(str(body?.message) || '机会投影响应无效')
  }
  const itemsRaw = Array.isArray(body.items) ? body.items : []
  return {
    items: itemsRaw.map((item) => mapOpportunityProjection(item as Record<string, unknown>)),
    total: Number(body.total) || 0,
    generated_at: str(body.generated_at ?? body.generatedAt),
  }
}

/** GET /api/opportunities/projections — list query (no stock_code). */
export async function fetchOpportunityProjections(
  q: OpportunityProjectionQuery = {},
): Promise<OpportunityProjectionsResult> {
  return fetchProjectionsEnvelope(buildSearchParams(q))
}

/** GET /api/opportunities/projections?stock_code=… — single-stock projection. */
export async function fetchOpportunityProjection(
  stockCode: string,
  q: OpportunityProjectionQuery = {},
): Promise<OpportunityProjection> {
  const code = String(stockCode || '').trim()
  if (!code) {
    throw new OpportunityProjectionNotFoundError('stock code required')
  }
  const result = await fetchProjectionsEnvelope(buildSearchParams({ ...q, stockCode: code }))
  if (result.items.length === 0) {
    throw new OpportunityProjectionNotFoundError('projection not found')
  }
  return result.items[0]
}
