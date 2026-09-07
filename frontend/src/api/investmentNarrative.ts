/**
 * Phase14-G0.5a/b — Investment Narrative API client (read-only).
 */

export type DiscoveryNarrative = {
  exists: boolean
  status: string
  signal_time?: string
  signal_price?: number
  reason?: string
  snapshot_id?: number
}

export type PlanOriginNarrative = {
  exists: boolean
  status: string
  plan_id?: number
  strategy?: string
  reason?: string
}

export type HoldingBasisNarrative = {
  exists: boolean
  status: string
  quantity?: number
  avg_cost?: number
}

export type ExitReviewOutcomeNarrative = {
  decision?: string
  review_time?: string
  reason?: string
  related_trade_plan_id?: number
}

export type ExitReviewNarrative = {
  exists: boolean
  status: string
  latest_outcome?: ExitReviewOutcomeNarrative | null
  reason_codes?: string[]
  summary?: string
}

export type PriceStoryNarrative = {
  status: string
  signal_price?: number
  current_price?: number
  vs_signal_pct?: number
}

export type InvestmentNarrative = {
  stock_code: string
  discovery: DiscoveryNarrative
  plan_origin: PlanOriginNarrative
  holding_basis: HoldingBasisNarrative
  exit_review: ExitReviewNarrative
  price_story: PriceStoryNarrative
  as_of?: string
  schema_version?: string
  data_source_note?: string
}

function mapDiscovery(raw: Record<string, unknown> | undefined): DiscoveryNarrative {
  if (!raw) return { exists: false, status: 'missing' }
  return {
    exists: !!raw.exists,
    status: String(raw.status || 'missing'),
    signal_time: raw.signal_time ? String(raw.signal_time) : undefined,
    signal_price: raw.signal_price != null ? Number(raw.signal_price) : undefined,
    reason: raw.reason ? String(raw.reason) : undefined,
    snapshot_id: raw.snapshot_id != null ? Number(raw.snapshot_id) : undefined,
  }
}

function mapPlanOrigin(raw: Record<string, unknown> | undefined): PlanOriginNarrative {
  if (!raw) return { exists: false, status: 'missing' }
  return {
    exists: !!raw.exists,
    status: String(raw.status || 'missing'),
    plan_id: raw.plan_id != null ? Number(raw.plan_id) : undefined,
    strategy: raw.strategy ? String(raw.strategy) : undefined,
    reason: raw.reason ? String(raw.reason) : undefined,
  }
}

function mapHoldingBasis(raw: Record<string, unknown> | undefined): HoldingBasisNarrative {
  if (!raw) return { exists: false, status: 'missing' }
  return {
    exists: !!raw.exists,
    status: String(raw.status || 'missing'),
    quantity: raw.quantity != null ? Number(raw.quantity) : undefined,
    avg_cost: raw.avg_cost != null ? Number(raw.avg_cost) : undefined,
  }
}

function mapExitReview(raw: Record<string, unknown> | undefined): ExitReviewNarrative {
  if (!raw) return { exists: false, status: 'missing' }
  const lo = (raw.latest_outcome || raw.latestOutcome) as Record<string, unknown> | undefined
  return {
    exists: !!raw.exists,
    status: String(raw.status || 'missing'),
    latest_outcome: lo
      ? {
          decision: lo.decision ? String(lo.decision) : undefined,
          review_time: lo.review_time ? String(lo.review_time) : lo.reviewTime ? String(lo.reviewTime) : undefined,
          reason: lo.reason ? String(lo.reason) : undefined,
          related_trade_plan_id:
            lo.related_trade_plan_id != null
              ? Number(lo.related_trade_plan_id)
              : lo.relatedTradePlanId != null
                ? Number(lo.relatedTradePlanId)
                : undefined,
        }
      : undefined,
    reason_codes: Array.isArray(raw.reason_codes) ? raw.reason_codes.map(String) : undefined,
    summary: raw.summary ? String(raw.summary) : undefined,
  }
}

function mapPriceStory(raw: Record<string, unknown> | undefined): PriceStoryNarrative {
  if (!raw) return { status: 'missing' }
  return {
    status: String(raw.status || 'missing'),
    signal_price: raw.signal_price != null ? Number(raw.signal_price) : undefined,
    current_price: raw.current_price != null ? Number(raw.current_price) : undefined,
    vs_signal_pct: raw.vs_signal_pct != null ? Number(raw.vs_signal_pct) : undefined,
  }
}

function mapNarrative(raw: Record<string, unknown>): InvestmentNarrative {
  return {
    stock_code: String(raw.stock_code || raw.stockCode || ''),
    discovery: mapDiscovery(raw.discovery as Record<string, unknown>),
    plan_origin: mapPlanOrigin(raw.plan_origin as Record<string, unknown>),
    holding_basis: mapHoldingBasis(raw.holding_basis as Record<string, unknown>),
    exit_review: mapExitReview(raw.exit_review as Record<string, unknown>),
    price_story: mapPriceStory(raw.price_story as Record<string, unknown>),
    as_of: raw.as_of ? String(raw.as_of) : raw.asOf ? String(raw.asOf) : undefined,
    schema_version: raw.schema_version ? String(raw.schema_version) : undefined,
    data_source_note: raw.data_source_note ? String(raw.data_source_note) : undefined,
  }
}

/** GET /api/investment/narrative/{stock_code} */
export async function fetchInvestmentNarrative(
  stockCode: string,
  accountId?: number,
): Promise<InvestmentNarrative> {
  const code = String(stockCode || '').trim()
  if (!code) {
    throw new Error('stock_code required')
  }
  const params = new URLSearchParams()
  if (accountId && accountId > 0) params.set('account_id', String(accountId))
  const qs = params.toString()
  const res = await fetch(
    `/api/investment/narrative/${encodeURIComponent(code)}${qs ? `?${qs}` : ''}`,
  )
  const body = (await res.json()) as Record<string, unknown>
  if (!res.ok || !body?.ok) {
    throw new Error(String(body?.message || `HTTP ${res.status}`))
  }
  const narr = (body.narrative || {}) as Record<string, unknown>
  return mapNarrative(narr)
}
