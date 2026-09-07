/**
 * Phase16-D4 / G1 — GET /api/opportunities/outcomes read-only client.
 * JSON field names match backend OutcomeProjection (snake_case).
 */
import type {
  OpportunityProjectionDecision,
  OpportunityProjectionOpportunity,
  OpportunityProjectionSignal,
} from './opportunityProjection.ts'

export const OUTCOME_STATUS = {
  OPEN: 'OPEN',
  CLOSED: 'CLOSED',
  NO_TRADE: 'NO_TRADE',
} as const

export type OutcomeEntry = {
  present: boolean
  buy_fill_id?: number
  buy_plan_id?: number
  buy_plan_item_id?: number
  entry_price?: number
  entry_qty?: number
  entry_fee?: number
  entry_date?: string
}

export type OutcomeExit = {
  present: boolean
  sell_fill_id?: number
  sell_plan_id?: number
  sell_plan_item_id?: number
  exit_price?: number
  exit_qty?: number
  exit_fee?: number
  exit_date?: string
  exit_channel?: string
  exit_reason_text?: string
}

export type OutcomePerformance = {
  entry_price?: number
  exit_price?: number
  quantity?: number
  realized_return_pct?: number
  holding_days?: number
}

export type OutcomeMetadata = {
  source_type: string
  quality: string
  missing: string[]
  fifo_policy?: string
  as_of: string
}

/** Unified read model; field names align with backend JSON tags. */
export type OutcomeProjection = {
  outcome_id: string
  opportunity_id: string
  stock_code: string
  stock_name?: string
  outcome_status: string
  signal: OpportunityProjectionSignal
  opportunity: OpportunityProjectionOpportunity
  decision: OpportunityProjectionDecision
  entry: OutcomeEntry
  exit: OutcomeExit
  performance: OutcomePerformance
  metadata: OutcomeMetadata
}

export type OpportunityOutcomeQuery = {
  tradeDate?: string
  stockCode?: string
  limit?: number
  status?: string
}

export type OpportunityOutcomesResult = {
  items: OutcomeProjection[]
  generated_at: string
}

export class OpportunityOutcomeNotFoundError extends Error {
  readonly status = 404

  constructor(message = 'outcome not found') {
    super(message)
    this.name = 'OpportunityOutcomeNotFoundError'
  }
}

function num(v: unknown): number | undefined {
  const n = Number(v)
  return Number.isFinite(n) ? n : undefined
}

function int(v: unknown): number | undefined {
  const n = Number(v)
  if (!Number.isFinite(n)) return undefined
  return Math.trunc(n)
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

function mapEntry(raw: Record<string, unknown> | undefined): OutcomeEntry {
  const r = raw && typeof raw === 'object' ? raw : {}
  const buyFillId = num(r.buy_fill_id ?? r.buyFillId)
  const buyPlanId = num(r.buy_plan_id ?? r.buyPlanId)
  const buyPlanItemId = num(r.buy_plan_item_id ?? r.buyPlanItemId)
  const entryPrice = num(r.entry_price ?? r.entryPrice)
  const entryQty = int(r.entry_qty ?? r.entryQty)
  const entryFee = num(r.entry_fee ?? r.entryFee)
  return {
    present: bool(r.present),
    buy_fill_id: buyFillId && buyFillId > 0 ? buyFillId : undefined,
    buy_plan_id: buyPlanId && buyPlanId > 0 ? buyPlanId : undefined,
    buy_plan_item_id: buyPlanItemId && buyPlanItemId > 0 ? buyPlanItemId : undefined,
    entry_price: entryPrice,
    entry_qty: entryQty,
    entry_fee: entryFee,
    entry_date: str(r.entry_date ?? r.entryDate) || undefined,
  }
}

function mapExit(raw: Record<string, unknown> | undefined): OutcomeExit {
  const r = raw && typeof raw === 'object' ? raw : {}
  const sellFillId = num(r.sell_fill_id ?? r.sellFillId)
  const sellPlanId = num(r.sell_plan_id ?? r.sellPlanId)
  const sellPlanItemId = num(r.sell_plan_item_id ?? r.sellPlanItemId)
  const exitPrice = num(r.exit_price ?? r.exitPrice)
  const exitQty = int(r.exit_qty ?? r.exitQty)
  const exitFee = num(r.exit_fee ?? r.exitFee)
  return {
    present: bool(r.present),
    sell_fill_id: sellFillId && sellFillId > 0 ? sellFillId : undefined,
    sell_plan_id: sellPlanId && sellPlanId > 0 ? sellPlanId : undefined,
    sell_plan_item_id: sellPlanItemId && sellPlanItemId > 0 ? sellPlanItemId : undefined,
    exit_price: exitPrice,
    exit_qty: exitQty,
    exit_fee: exitFee,
    exit_date: str(r.exit_date ?? r.exitDate) || undefined,
    exit_channel: str(r.exit_channel ?? r.exitChannel) || undefined,
    exit_reason_text: str(r.exit_reason_text ?? r.exitReasonText) || undefined,
  }
}

function mapPerformance(raw: Record<string, unknown> | undefined): OutcomePerformance {
  const r = raw && typeof raw === 'object' ? raw : {}
  const entryPrice = num(r.entry_price ?? r.entryPrice)
  const exitPrice = num(r.exit_price ?? r.exitPrice)
  const quantity = int(r.quantity)
  const realizedReturnPct = num(r.realized_return_pct ?? r.realizedReturnPct)
  const holdingDays = int(r.holding_days ?? r.holdingDays)
  return {
    entry_price: entryPrice,
    exit_price: exitPrice,
    quantity,
    realized_return_pct: realizedReturnPct,
    holding_days: holdingDays,
  }
}

function mapMetadata(raw: Record<string, unknown> | undefined): OutcomeMetadata {
  const r = raw && typeof raw === 'object' ? raw : {}
  const missingRaw = r.missing
  const missing = Array.isArray(missingRaw) ? missingRaw.map((m) => String(m)) : []
  return {
    source_type: str(r.source_type ?? r.sourceType),
    quality: str(r.quality),
    missing,
    fifo_policy: str(r.fifo_policy ?? r.fifoPolicy) || undefined,
    as_of: str(r.as_of ?? r.asOf),
  }
}

/** Map one backend OutcomeProjection JSON object. */
export function mapOutcomeProjection(raw: Record<string, unknown>): OutcomeProjection {
  return {
    outcome_id: str(raw.outcome_id ?? raw.outcomeId),
    opportunity_id: str(raw.opportunity_id ?? raw.opportunityId),
    stock_code: str(raw.stock_code ?? raw.stockCode),
    stock_name: str(raw.stock_name ?? raw.stockName) || undefined,
    outcome_status: str(raw.outcome_status ?? raw.outcomeStatus),
    signal: mapSignal(raw.signal as Record<string, unknown> | undefined),
    opportunity: mapOpportunityBlock(raw.opportunity as Record<string, unknown> | undefined),
    decision: mapDecision(raw.decision as Record<string, unknown> | undefined),
    entry: mapEntry(raw.entry as Record<string, unknown> | undefined),
    exit: mapExit(raw.exit as Record<string, unknown> | undefined),
    performance: mapPerformance(raw.performance as Record<string, unknown> | undefined),
    metadata: mapMetadata(raw.metadata as Record<string, unknown> | undefined),
  }
}

function buildSearchParams(q: OpportunityOutcomeQuery = {}): URLSearchParams {
  const params = new URLSearchParams()
  if (q.tradeDate?.trim()) params.set('trade_date', q.tradeDate.trim())
  if (q.stockCode?.trim()) params.set('stock_code', q.stockCode.trim())
  if (q.limit != null && q.limit > 0) params.set('limit', String(q.limit))
  if (q.status?.trim()) params.set('status', q.status.trim())
  return params
}

async function fetchOutcomesEnvelope(params: URLSearchParams): Promise<OpportunityOutcomesResult> {
  const res = await fetch(`/api/opportunities/outcomes?${params.toString()}`)
  const body = (await res.json().catch(() => ({}))) as Record<string, unknown>
  if (res.status === 404 || (body && body.ok === false && Number(body.code) === 404)) {
    throw new OpportunityOutcomeNotFoundError(str(body?.message) || 'outcome not found')
  }
  if (!res.ok) {
    throw new Error(str(body?.message) || `机会结果请求失败: HTTP ${res.status}`)
  }
  if (!body?.ok) {
    throw new Error(str(body?.message) || '机会结果响应无效')
  }
  const itemsRaw = Array.isArray(body.items) ? body.items : []
  return {
    items: itemsRaw.map((item) => mapOutcomeProjection(item as Record<string, unknown>)),
    generated_at: str(body.generated_at ?? body.generatedAt),
  }
}

/** GET /api/opportunities/outcomes — list query (no stock_code). */
export async function fetchOpportunityOutcomes(
  q: OpportunityOutcomeQuery = {},
): Promise<OpportunityOutcomesResult> {
  return fetchOutcomesEnvelope(buildSearchParams(q))
}

/** GET /api/opportunities/outcomes?stock_code=… — single-stock outcomes (may return multiple FIFO legs). */
export async function fetchOpportunityOutcome(
  stockCode: string,
  q: Omit<OpportunityOutcomeQuery, 'stockCode'> = {},
): Promise<OpportunityOutcomesResult> {
  const code = String(stockCode || '').trim()
  if (!code) {
    throw new OpportunityOutcomeNotFoundError('stock code required')
  }
  return fetchOutcomesEnvelope(buildSearchParams({ ...q, stockCode: code }))
}
